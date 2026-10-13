package service

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// passwordSetter 只暴露「不校验旧密码、直接重设」这一个方法
// 不依赖整个 *UserService，单测换个假实现就能跑
type passwordSetter interface {
	ResetPassword(ctx context.Context, userId valobj.Uid, newPassword string) error
}

// PasswordResetService 处理找回密码：发链接凭证、限制发信间隔、确认后重设密码
// 发邮件与拼链接由 app 层负责，本服务只返回 token
type PasswordResetService struct {
	resetRepo   repository.PasswordResetRepository
	userRepo    repository.UserRepository
	userService passwordSetter
}

func NewPasswordResetService(
	resetRepo repository.PasswordResetRepository,
	userRepo repository.UserRepository,
	userService passwordSetter,
) *PasswordResetService {
	return &PasswordResetService{
		resetRepo:   resetRepo,
		userRepo:    userRepo,
		userService: userService,
	}
}

// PasswordResetDispatch 发起找回的结果，token 只用于拼邮件里的链接，不进库
// Sent 为 false 表示这个邮箱没有账号，接口照常返回成功，但不发信也不写 pending
// Rollback 在 app 层发信失败时调用，撤销本次占用的发送间隔，允许立即重试
type PasswordResetDispatch struct {
	Email     string
	LinkToken string
	Sent      bool
	Rollback  func(ctx context.Context)
}

// Start 发起找回：占发送间隔 → 查账号 → 账号存在才写 pending 并返回 token
// 账号不存在也一样返回成功（Sent=false），别让调用方有机会据此判断邮箱是否注册
func (s *PasswordResetService) Start(
	ctx context.Context,
	rawEmail string,
) (*PasswordResetDispatch, error) {
	email := entity.NormalizeEmail(rawEmail)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}

	// 不管账号存不存在都先占发送间隔。只给存在的账号限流的话，
	// 反复请求就能从「有没有被限流」反推邮箱是不是注册过
	if err := s.resetRepo.StartSendCooldown(ctx, email); err != nil {
		return nil, err
	}
	rollback := func(ctx context.Context) { s.clearCooldown(ctx, email) }

	if _, err := s.userRepo.GetByEmail(ctx, email); err != nil {
		if errors.Is(err, identerr.ErrUserNotFound) {
			// 不写 pending 也不发信，但发送间隔留着，表现和账号存在时一样
			return &PasswordResetDispatch{Rollback: rollback}, nil
		}

		rollback(ctx)

		return nil, errors.WithMessage(err, "lookup account failed")
	}

	pending, previousLinkTokenHash, err := s.preparePending(ctx, email)
	if err != nil {
		rollback(ctx)

		return nil, err
	}

	linkToken, err := pending.IssueCredentials()
	if err != nil {
		rollback(ctx)

		return nil, errors.WithMessage(err, "issue password reset credentials failed")
	}

	if err := s.resetRepo.Save(ctx, pending, previousLinkTokenHash); err != nil {
		rollback(ctx)

		return nil, errors.WithMessage(err, "save password reset pending failed")
	}

	return &PasswordResetDispatch{
		Email:     email,
		LinkToken: linkToken,
		Sent:      true,
		Rollback:  rollback,
	}, nil
}

// ConfirmLink 链接确认：token 本身就是凭证
// 重发后旧链接的索引已删掉，旧链接立即失效
func (s *PasswordResetService) ConfirmLink(
	ctx context.Context,
	linkToken, newPassword string,
) (valobj.Uid, error) {
	pending, err := s.resetRepo.GetByLinkToken(ctx, linkToken)
	if err != nil {
		return valobj.Uid{}, err
	}

	return s.confirm(ctx, pending, newPassword)
}

// preparePending 同邮箱已有 pending 就复用，只轮换链接 token，没有则新建
// 返回旧的链接哈希，落库时顺带删掉它，旧链接立即失效
func (s *PasswordResetService) preparePending(
	ctx context.Context,
	email string,
) (*entity.PasswordResetPending, string, error) {
	existing, err := s.resetRepo.GetByEmail(ctx, email)
	if err == nil {
		return existing, existing.LinkTokenHash, nil
	}
	// 只有记录不存在才算首次请求，读缓存出错要直接返回，不能当成首次
	if !errors.Is(err, identerr.ErrVerificationExpired) {
		return nil, "", err
	}

	return entity.NewPasswordResetPending(email), "", nil
}

// confirm 取账号 → 改密码 → 删凭证
// 改密码失败就不删凭证，用户能用同一个链接重试
func (s *PasswordResetService) confirm(
	ctx context.Context,
	pending *entity.PasswordResetPending,
	newPassword string,
) (valobj.Uid, error) {
	user, err := s.userRepo.GetByEmail(ctx, pending.Email)
	if err != nil {
		// 账号在发起后被删了，凭证不再指向任何账号，按凭证过期处理
		if errors.Is(err, identerr.ErrUserNotFound) {
			return valobj.Uid{}, identerr.ErrVerificationExpired
		}

		return valobj.Uid{}, errors.WithMessage(err, "lookup account failed")
	}

	if err := s.userService.ResetPassword(ctx, user.Id, newPassword); err != nil {
		return valobj.Uid{}, err
	}

	if err := s.resetRepo.DeleteByEmail(ctx, pending.Email); err != nil {
		// 密码已经改完，凭证删不掉就留着，最多等 TTL 过期，不影响这次重置
		slog.WarnContext(ctx, "delete password reset pending failed",
			slog.String("email", pending.Email),
			slog.Any("err", err),
		)
	}

	return user.Id, nil
}

func (s *PasswordResetService) clearCooldown(ctx context.Context, email string) {
	if err := s.resetRepo.ClearSendCooldown(ctx, email); err != nil {
		slog.WarnContext(ctx, "clear password reset send cooldown failed", slog.Any("err", err))
	}
}

package service

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const signupTicketLength = 32

// SignupService 处理邮箱注册：生成凭证、限制发邮件间隔、验证后建号
// 发邮件与拼链接由 app 层负责，本服务只返回 token 与验证码
type SignupService struct {
	signupRepo  repository.SignupRepository
	userRepo    repository.UserRepository
	userService *UserService
	hasher      adapter.PasswordHasher
}

func NewSignupService(
	signupRepo repository.SignupRepository,
	userRepo repository.UserRepository,
	userService *UserService,
	hasher adapter.PasswordHasher,
) *SignupService {
	return &SignupService{
		signupRepo:  signupRepo,
		userRepo:    userRepo,
		userService: userService,
		hasher:      hasher,
	}
}

// SignupDispatch 给 app 层返回凭证时的结果，token 与验证码只用于发邮件，不进库
// Ticket 仅首次注册返回，重复注册为空，前端保留旧 ticket
// Rollback 在 app 层发邮件失败时调用，撤销本次写入的发送间隔，允许立即重试
type SignupDispatch struct {
	Email     string
	Ticket    string
	LinkToken string
	Code      string
	Rollback  func(ctx context.Context)
}

// Start 邮箱注册第一步：校验 → 邮箱未占用 → 哈希密码 → 沿用或新建 pending → 返回凭证
func (s *SignupService) Start(ctx context.Context, email, password string) (*SignupDispatch, error) {
	email, passwordHash, err := s.prepareCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}

	pending, previousLinkTokenHash, ticket, err := s.preparePending(ctx, email, passwordHash)
	if err != nil {
		return nil, err
	}

	dispatch, err := s.dispatch(ctx, pending, previousLinkTokenHash)
	if err != nil {
		return nil, err
	}
	dispatch.Ticket = ticket

	return dispatch, nil
}

// DirectSignup 免验证注册：邮箱视为已验证
func (s *SignupService) DirectSignup(
	ctx context.Context,
	email, password string,
) (*entity.UserAggregate, error) {
	email, passwordHash, err := s.prepareCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}

	return s.userService.createVerifiedAccount(ctx, email, passwordHash)
}

// prepareCredentials 两条注册入口共用的校验，返回统一格式的邮箱与密码哈希
func (s *SignupService) prepareCredentials(
	ctx context.Context,
	email, password string,
) (normalizedEmail, passwordHash string, err error) {
	normalizedEmail = entity.NormalizeEmail(email)
	if err := entity.ValidateEmail(normalizedEmail); err != nil {
		return "", "", err
	}
	if err := entity.ValidatePassword(password); err != nil {
		return "", "", err
	}

	if err := s.ensureEmailAvailable(ctx, normalizedEmail); err != nil {
		return "", "", err
	}

	passwordHash, err = s.hasher.Hash(ctx, password)
	if err != nil {
		return "", "", errors.WithMessage(err, "hash password failed")
	}

	return normalizedEmail, passwordHash, nil
}

// Resend 重发验证信
func (s *SignupService) Resend(ctx context.Context, email string) (*SignupDispatch, error) {
	email = entity.NormalizeEmail(email)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}

	if err := s.ensureEmailAvailable(ctx, email); err != nil {
		return nil, err
	}

	pending, err := s.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return s.dispatch(ctx, pending, pending.LinkTokenHash)
}

// VerifyLink 链接验证：用邮件里的 token 取出 pending 再建号
func (s *SignupService) VerifyLink(
	ctx context.Context,
	linkToken, ticket string,
) (*entity.UserAggregate, error) {
	pending, err := s.signupRepo.GetByLinkToken(ctx, linkToken)
	if err != nil {
		return nil, err
	}

	return s.activate(ctx, pending, ticket)
}

// VerifyCode 验证码验证：先按 email 取 pending，码不匹配只累加 attempts
// 达到 SignupMaxCodeAttempts 就作废整个 pending，用户需重新注册
func (s *SignupService) VerifyCode(
	ctx context.Context,
	email, code, ticket string,
) (*entity.UserAggregate, error) {
	email = entity.NormalizeEmail(email)

	pending, err := s.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if !pending.MatchesCode(code) {
		attempts, incrErr := s.signupRepo.IncrAttempts(ctx, email)
		if incrErr != nil {
			slog.WarnContext(ctx, "incr signup attempts failed", slog.Any("err", incrErr))
		}

		if attempts >= entity.SignupMaxCodeAttempts {
			if delErr := s.signupRepo.DeleteByEmail(ctx, email); delErr != nil {
				slog.WarnContext(ctx, "drop signup pending after too many attempts failed", slog.Any("err", delErr))
			}

			return nil, identerr.ErrVerificationAttemptsExceeded
		}

		return nil, identerr.ErrVerificationCodeMismatch
	}

	return s.activate(ctx, pending, ticket)
}

// ensureEmailAvailable 已注册的邮箱不再进入验证流程
func (s *SignupService) ensureEmailAvailable(ctx context.Context, email string) error {
	if _, err := s.userRepo.GetByEmail(ctx, email); err == nil {
		return identerr.ErrEmailTaken
	} else if !errors.Is(err, identerr.ErrUserNotFound) {
		return errors.WithMessage(err, "check email failed")
	}

	return nil
}

// preparePending 重复注册时保留首次的密码与 ticket，只轮换链接 token 和验证码
func (s *SignupService) preparePending(
	ctx context.Context,
	email, passwordHash string,
) (pending *entity.SignupPending, previousLinkTokenHash, ticket string, err error) {
	existing, err := s.signupRepo.GetByEmail(ctx, email)
	if err == nil {
		return existing, existing.LinkTokenHash, "", nil
	}
	if !errors.Is(err, identerr.ErrVerificationExpired) {
		return nil, "", "", err
	}

	ticket, err = idp.RandomString(signupTicketLength)
	if err != nil {
		return nil, "", "", errors.WithMessage(err, "generate signup ticket failed")
	}

	return entity.NewSignupPending(email, passwordHash, ticket), "", ticket, nil
}

// dispatch 生成凭证 → 写发送间隔 → 写库
// 先写发送间隔再写库，否则限流请求会换掉链接却没发邮件，写库失败立即撤销间隔
func (s *SignupService) dispatch(
	ctx context.Context,
	pending *entity.SignupPending,
	previousLinkTokenHash string,
) (*SignupDispatch, error) {
	linkToken, code, err := pending.IssueCredentials()
	if err != nil {
		return nil, errors.WithMessage(err, "issue signup credentials failed")
	}

	if err := s.signupRepo.StartSendCooldown(ctx, pending.Email); err != nil {
		return nil, err
	}
	rollback := func(ctx context.Context) { s.clearCooldown(ctx, pending.Email) }

	if err := s.signupRepo.Save(ctx, pending, previousLinkTokenHash); err != nil {
		rollback(ctx)
		return nil, errors.WithMessage(err, "save signup pending failed")
	}

	return &SignupDispatch{
		Email:     pending.Email,
		LinkToken: linkToken,
		Code:      code,
		Rollback:  rollback,
	}, nil
}

// activate 邮箱验证通过后建号：只有发起注册的浏览器完成验证才写入密码
func (s *SignupService) activate(
	ctx context.Context,
	pending *entity.SignupPending,
	ticket string,
) (*entity.UserAggregate, error) {
	passwordHash := ""
	if pending.MatchesTicket(ticket) {
		passwordHash = pending.PasswordHash
	}

	agg, err := s.userService.createVerifiedAccount(ctx, pending.Email, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := s.signupRepo.DeleteByEmail(ctx, pending.Email); err != nil {
		slog.WarnContext(ctx, "delete signup pending failed",
			slog.String("email", pending.Email),
			slog.Any("err", err),
		)
	}

	return agg, nil
}

func (s *SignupService) clearCooldown(ctx context.Context, email string) {
	if err := s.signupRepo.ClearSendCooldown(ctx, email); err != nil {
		slog.WarnContext(ctx, "clear signup send cooldown failed", slog.Any("err", err))
	}
}

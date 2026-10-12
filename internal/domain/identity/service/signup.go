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

// SignupService 编排邮箱注册：签发验证凭证、占发信冷却、消费 pending 建号。
// 邮件发送与链接拼接属于投递，由 app 层负责，服务只返回待发送的明文凭证。
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

// SignupDispatch 一次凭证下发的结果：明文 token/code 只交给 app 层发信，不进库；
// Ticket 仅首次注册下发（重复注册为空，前端保留旧票据）；
// Rollback 在 app 层发信失败时调用，撤销本次占用的发信冷却，让用户可立即重试。
type SignupDispatch struct {
	Email     string
	Ticket    string
	LinkToken string
	Code      string
	Rollback  func(ctx context.Context)
}

// Start 邮箱注册第一步：校验 → 邮箱未占用 → 哈希口令 → 复用或新建 pending → 下发凭证。
func (s *SignupService) Start(ctx context.Context, email, password string) (*SignupDispatch, error) {
	email = entity.NormalizeEmail(email)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := entity.ValidatePassword(password); err != nil {
		return nil, err
	}

	if err := s.ensureEmailAvailable(ctx, email); err != nil {
		return nil, err
	}

	passwordHash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return nil, errors.WithMessage(err, "hash password failed")
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

// VerifyLink 链接验证：明文 token 换出 pending，再建号。
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

// VerifyCode 验证码验证：先按 email 取 pending；码不匹配只累加 attempts，
// 达到 SignupMaxCodeAttempts 就作废整个 pending（用户需重新注册）。
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

// ensureEmailAvailable 已注册的邮箱不再进入验证流程。
func (s *SignupService) ensureEmailAvailable(ctx context.Context, email string) error {
	if _, err := s.userRepo.GetByEmail(ctx, email); err == nil {
		return identerr.ErrEmailTaken
	} else if !errors.Is(err, identerr.ErrUserNotFound) {
		return errors.WithMessage(err, "check email failed")
	}

	return nil
}

// preparePending 重复注册时保留首次的口令与票据，只轮换两条验证凭证。
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

// dispatch 签发凭证 → 占发信冷却 → 落库。占位在落库前，避免被限流的请求轮换了
// 链接却没发信；落库失败时立即撤销冷却，允许用户重试。
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

// activate 邮箱被证明后建号：只有发起注册的那个浏览器完成验证，才把惰性口令激活。
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

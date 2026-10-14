package service

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// SignupService 处理邮箱注册：生成凭证、限制发邮件间隔、验证后创建账号
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

// SignupDispatch 给 app 层返回凭证时的结果，验证码只用于发邮件，不进库
// Ticket 非空时由 app 层写注册票据 cookie，已持有有效票据的重复注册为空
// Rollback 在 app 层发邮件失败时调用，撤销本次写入的发送间隔，允许立即重试
type SignupDispatch struct {
	Email    string
	Ticket   string
	Code     string
	Rollback func(ctx context.Context)
}

// Start 邮箱注册第一步：校验 → 邮箱未占用 → 哈希密码 → 沿用或新建 state → 返回凭证
// ticket 为请求携带的注册票据，票据与密码必须同源，各分支：
//
//   - 无待验证记录：新建记录 + 新票据，Ticket 非空，由 app 层写 cookie；
//   - 已有记录且票据匹配：沿用首次的密码，Ticket 为空，不动 cookie；
//   - 已有记录但票据缺失或不匹配（上次发验证邮件失败没拿到 cookie、换浏览器、清过 cookie）：
//     轮换票据并接管本次提交的密码，返回新票据，旧票据立即失效；
//   - 撞 60s 冷却：记录不落库，轮换只留在内存里，旧票据与旧 cookie 依然有效。
func (s *SignupService) Start(ctx context.Context, email, password, ticket string) (*SignupDispatch, error) {
	email, passwordHash, err := s.prepareCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}

	state, issuedTicket, err := s.prepareState(ctx, email, passwordHash, ticket)
	if err != nil {
		return nil, err
	}

	dispatch, err := s.dispatch(ctx, state)
	if err != nil {
		return nil, err
	}
	dispatch.Ticket = issuedTicket

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
// 只重置验证码，不碰注册票据：重发不携带密码，轮换等于替别人接管这条记录
func (s *SignupService) Resend(ctx context.Context, email string) (*SignupDispatch, error) {
	email = entity.NormalizeEmail(email)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}

	if err := s.ensureEmailAvailable(ctx, email); err != nil {
		return nil, err
	}

	state, err := s.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return s.dispatch(ctx, state)
}

// VerifyCode 验证码验证：先按 email 取 state，码不匹配只累加 attempts
// 达到 SignupMaxCodeAttempts 就作废整个 state，用户需重新注册
func (s *SignupService) VerifyCode(
	ctx context.Context,
	email, code, ticket string,
) (*entity.UserAggregate, error) {
	email = entity.NormalizeEmail(email)

	state, err := s.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if !state.MatchesCode(code) {
		attempts, incrErr := s.signupRepo.IncrAttempts(ctx, email)
		if incrErr != nil {
			slog.WarnContext(ctx, "incr signup attempts failed", slog.Any("err", incrErr))
		}

		if attempts >= entity.SignupMaxCodeAttempts {
			if delErr := s.signupRepo.DeleteByEmail(ctx, email); delErr != nil {
				slog.WarnContext(ctx, "drop signup state after too many attempts failed", slog.Any("err", delErr))
			}

			return nil, identerr.ErrVerificationAttemptsExceeded
		}

		return nil, identerr.ErrVerificationCodeMismatch
	}

	return s.activate(ctx, state, ticket)
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

// prepareState 重复注册时保留首次的密码与 ticket，只轮换验证码
// 浏览器手里没有有效票据则改由它接管记录：票据与密码一起换，两者必须同源
func (s *SignupService) prepareState(
	ctx context.Context,
	email, passwordHash, ticket string,
) (state *entity.SignupState, issuedTicket string, err error) {
	existing, err := s.signupRepo.GetByEmail(ctx, email)
	if err == nil {
		if existing.MatchesTicket(ticket) {
			return existing, "", nil
		}

		issuedTicket, err = existing.RotateCredentials(passwordHash)
		if err != nil {
			return nil, "", errors.WithMessage(err, "rotate signup credentials failed")
		}

		return existing, issuedTicket, nil
	}
	if !errors.Is(err, identerr.ErrVerificationExpired) {
		return nil, "", err
	}

	issuedTicket, err = entity.NewSignupTicket()
	if err != nil {
		return nil, "", errors.WithMessage(err, "generate signup ticket failed")
	}

	return entity.NewSignupState(email, passwordHash, issuedTicket), issuedTicket, nil
}

// dispatch 生成验证码 → 写发送间隔 → 落库
// 先写发送间隔再落库，否则限流请求会换掉验证码却没发邮件，落库失败立即撤销间隔
func (s *SignupService) dispatch(
	ctx context.Context,
	state *entity.SignupState,
) (*SignupDispatch, error) {
	code, err := state.IssueCredentials()
	if err != nil {
		return nil, errors.WithMessage(err, "issue signup credentials failed")
	}

	if err := s.signupRepo.StartSendCooldown(ctx, state.Email); err != nil {
		return nil, err
	}
	rollback := func(ctx context.Context) { s.clearCooldown(ctx, state.Email) }

	if err := s.signupRepo.Save(ctx, state); err != nil {
		rollback(ctx)
		return nil, errors.WithMessage(err, "save signup state failed")
	}

	return &SignupDispatch{
		Email:    state.Email,
		Code:     code,
		Rollback: rollback,
	}, nil
}

// activate 邮箱验证通过后创建账号：只有发起注册的浏览器完成验证才写入密码
func (s *SignupService) activate(
	ctx context.Context,
	state *entity.SignupState,
	ticket string,
) (*entity.UserAggregate, error) {
	passwordHash := ""
	if state.MatchesTicket(ticket) {
		passwordHash = state.PasswordHash
	}

	agg, err := s.userService.createVerifiedAccount(ctx, state.Email, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := s.signupRepo.DeleteByEmail(ctx, state.Email); err != nil {
		slog.WarnContext(ctx, "delete signup state failed",
			slog.String("email", state.Email),
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

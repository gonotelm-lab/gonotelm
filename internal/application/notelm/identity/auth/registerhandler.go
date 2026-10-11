package auth

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const signupTicketLength = 32

// RegisterHandler 邮箱注册第一步：只写待验证记录并发出验证信，不建账号、不签发会话。
type RegisterHandler struct {
	signupRepo    identrepo.SignupRepository
	userRepo      identrepo.UserRepository
	hasher        adapter.PasswordHasher
	mailer        adapter.Mailer
	verifyURLBase string
}

func NewRegisterHandler(
	signupRepo identrepo.SignupRepository,
	userRepo identrepo.UserRepository,
	hasher adapter.PasswordHasher,
	mailer adapter.Mailer,
	verifyURLBase string,
) *RegisterHandler {
	return &RegisterHandler{
		signupRepo:    signupRepo,
		userRepo:      userRepo,
		hasher:        hasher,
		mailer:        mailer,
		verifyURLBase: verifyURLBase,
	}
}

type RegisterCommand struct {
	Email    string
	Password string
}

type RegisterResult struct {
	// Ticket 非空时由 interfaces 层写进 _gnlm_signup_ticket；
	// 重复注册会保留首次票据，不重置 cookie。
	Ticket string
}

func (h *RegisterHandler) Handle(ctx context.Context, cmd *RegisterCommand) (*RegisterResult, error) {
	email := entity.NormalizeEmail(cmd.Email)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := entity.ValidatePassword(cmd.Password); err != nil {
		return nil, err
	}

	if _, err := h.userRepo.GetByEmail(ctx, email); err == nil {
		return nil, identerr.ErrEmailTaken
	} else if !errors.Is(err, identerr.ErrUserNotFound) {
		return nil, errors.WithMessage(err, "check email failed")
	}

	passwordHash, err := h.hasher.Hash(ctx, cmd.Password)
	if err != nil {
		return nil, errors.WithMessage(err, "hash password failed")
	}

	pending, previousLinkTokenHash, ticket, err := h.preparePending(ctx, email, passwordHash)
	if err != nil {
		return nil, err
	}

	linkToken, code, err := pending.IssueCredentials()
	if err != nil {
		return nil, errors.WithMessage(err, "issue signup credentials failed")
	}

	if err := h.signupRepo.Save(ctx, pending, previousLinkTokenHash); err != nil {
		return nil, errors.WithMessage(err, "save signup pending failed")
	}

	link := buildVerifyLink(h.verifyURLBase, linkToken)
	if err := h.mailer.SendEmailVerification(ctx, email, link, code); err != nil {
		return nil, errors.Wrapf(identerr.ErrMailSendFailed, "send verification mail: %s", err.Error())
	}

	return &RegisterResult{Ticket: ticket}, nil
}

// preparePending 重复注册时保留首次的口令与票据，只轮换两条验证凭证。
func (h *RegisterHandler) preparePending(
	ctx context.Context,
	email, passwordHash string,
) (pending *entity.SignupPending, previousLinkTokenHash, ticket string, err error) {
	existing, err := h.signupRepo.GetByEmail(ctx, email)
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

// buildVerifyLink 验证链接落到前端页面，token 放 URL 片段，不进服务端日志也不进 Referer。
func buildVerifyLink(base, linkToken string) string {
	return strings.TrimRight(base, "/") + "#token=" + linkToken
}

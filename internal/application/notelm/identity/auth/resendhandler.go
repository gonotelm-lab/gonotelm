package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// ResendHandler 重发验证信：只重置两条验证凭证，口令与票据保持不变。
type ResendHandler struct {
	signupRepo    identrepo.SignupRepository
	userRepo      identrepo.UserRepository
	mailer        adapter.Mailer
	verifyURLBase string
}

func NewResendHandler(
	signupRepo identrepo.SignupRepository,
	userRepo identrepo.UserRepository,
	mailer adapter.Mailer,
	verifyURLBase string,
) *ResendHandler {
	return &ResendHandler{
		signupRepo:    signupRepo,
		userRepo:      userRepo,
		mailer:        mailer,
		verifyURLBase: verifyURLBase,
	}
}

type ResendCommand struct {
	Email string
}

type ResendResult struct{}

func (h *ResendHandler) Handle(ctx context.Context, cmd *ResendCommand) (*ResendResult, error) {
	email := entity.NormalizeEmail(cmd.Email)
	if err := entity.ValidateEmail(email); err != nil {
		return nil, err
	}

	if _, err := h.userRepo.GetByEmail(ctx, email); err == nil {
		return nil, identerr.ErrEmailTaken
	} else if !errors.Is(err, identerr.ErrUserNotFound) {
		return nil, errors.WithMessage(err, "check email failed")
	}

	pending, err := h.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	previousLinkTokenHash := pending.LinkTokenHash
	linkToken, code, err := pending.IssueCredentials()
	if err != nil {
		return nil, errors.WithMessage(err, "issue signup credentials failed")
	}

	if err := h.signupRepo.Save(ctx, pending, previousLinkTokenHash); err != nil {
		return nil, errors.WithMessage(err, "save signup pending failed")
	}

	link := buildVerifyLink(h.verifyURLBase, linkToken)
	if err := h.mailer.SendEmailVerification(ctx, email, link, code); err != nil {
		return nil, errors.WithMessage(err, "send verification mail failed")
	}

	return &ResendResult{}, nil
}

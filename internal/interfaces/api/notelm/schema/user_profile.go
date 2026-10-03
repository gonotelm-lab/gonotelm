package schema

import (
	"strings"
	"unicode/utf8"

	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateMeRequest struct {
	Nickname *string `json:"nickname"`
}

func (r *UpdateMeRequest) Validate() error {
	if r.Nickname == nil {
		return errors.ErrParams.Msg("no profile field to update")
	}

	nickname := strings.TrimSpace(*r.Nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > identityentity.MaxUserNickNameRune {
		return errors.ErrParams.Msg("invalid nickname")
	}
	r.Nickname = &nickname

	return nil
}

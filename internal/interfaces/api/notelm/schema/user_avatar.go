package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

const (
	AvatarFormField = "avatar_file"

	MaxAvatarBytes     = int64(entity.MaxAvatarBytes)
	MaxAvatarBodyBytes = MaxAvatarBytes + 4*1024
)

type UpdateAvatarResponse struct {
	AvatarUrl string `json:"avatar_url"`
}

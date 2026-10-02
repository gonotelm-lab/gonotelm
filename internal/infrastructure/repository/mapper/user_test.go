package mapper

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRoundTrip_WithAvatar(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
	user.SetEmail("nick@example.com")

	avatar, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/"+uuid.NewV4().String(), true)
	require.NoError(t, err)
	user.SetAvatar(avatar)

	sch := UserToSchema(user)
	assert.NotEmpty(t, sch.Avatar, "avatar 列存的是编码形态")
	assert.NotContains(t, sch.Avatar, "users/avatar/", "编码形态里不应出现明文路径")

	back, err := UserFromSchema(sch)
	require.NoError(t, err)

	assert.Equal(t, user.Id, back.Id)
	assert.Equal(t, user.Email, back.Email)
	assert.Equal(t, user.Nickname, back.Nickname)
	assert.Equal(t, user.Provider, back.Provider)
	assert.Equal(t, user.Sub, back.Sub)
	assert.Equal(t, avatar, back.Avatar)
	assert.True(t, back.Avatar.IsPublic)
}

func TestUserRoundTrip_WithoutAvatar(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")

	sch := UserToSchema(user)
	assert.Empty(t, sch.Avatar, "无头像时 avatar 列为空串")

	back, err := UserFromSchema(sch)
	require.NoError(t, err)
	assert.False(t, back.Avatar.Valid())
}

func TestUserFromSchema_RejectsUndecodableAvatar(t *testing.T) {
	// 旧数据（裸 URL）不再兼容，读取即报错
	_, err := UserFromSchema(&schema.User{Avatar: "https://example.com/a.png"})
	assert.Error(t, err)
}

package mapper

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
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
	assert.NotEmpty(t, sch.Avatar, "avatar column stores the encoded form")
	assert.NotContains(t, sch.Avatar, "users/avatar/", "the encoded form must not contain the plain path")

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
	assert.Empty(t, sch.Avatar, "avatar column is empty when the user has no avatar")

	back, err := UserFromSchema(sch)
	require.NoError(t, err)
	assert.False(t, back.Avatar.Valid())
}

func TestUserFromSchema_RejectsUndecodableAvatar(t *testing.T) {
	_, err := UserFromSchema(&schema.User{Avatar: "https://example.com/a.png"})
	assert.Error(t, err)
}

func TestUserCacheSchemaRoundTrip(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
	user.SetEmail("nick@example.com")

	avatar, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/"+uuid.NewV4().String(), true)
	require.NoError(t, err)
	user.SetAvatar(avatar)

	sch := UserToCacheSchema(user)
	assert.Equal(t, user.Id.String(), sch.Id, "the cached id is the string form")
	assert.NotContains(t, sch.Avatar, "users/avatar/", "the encoded form must not contain the plain path")

	back, err := UserFromCacheSchema(sch)
	require.NoError(t, err)

	assert.Equal(t, user.Id, back.Id)
	assert.Equal(t, user.Email, back.Email)
	assert.Equal(t, user.Nickname, back.Nickname)
	assert.Equal(t, user.Status, back.Status)
	assert.Equal(t, user.Provider, back.Provider)
	assert.Equal(t, user.Sub, back.Sub)
	assert.Equal(t, avatar, back.Avatar)
	assert.True(t, back.Avatar.IsPublic)
	assert.Equal(t, user.CreatedAt.Value(), back.CreatedAt.Value())
	assert.Equal(t, user.UpdatedAt.Value(), back.UpdatedAt.Value())
}

func TestUserCacheSchemaRoundTrip_WithoutAvatar(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")

	sch := UserToCacheSchema(user)
	assert.Empty(t, sch.Avatar, "avatar is empty when the user has no avatar")

	back, err := UserFromCacheSchema(sch)
	require.NoError(t, err)
	assert.False(t, back.Avatar.Valid())
}

func TestUserFromCacheSchema_RejectsBadInput(t *testing.T) {
	_, err := UserFromCacheSchema(&cacheschema.User{Id: "not-a-ulid"})
	assert.Error(t, err)

	_, err = UserFromCacheSchema(&cacheschema.User{
		Id:     valobj.NewUid().String(),
		Avatar: "https://example.com/a.png",
	})
	assert.Error(t, err)
}

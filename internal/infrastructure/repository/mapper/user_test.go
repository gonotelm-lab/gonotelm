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

func newTestUser() *identityentity.User {
	return identityentity.NewUser("nick@example.com", "nick", valobj.NewTime())
}

func TestUserRoundTrip_WithAvatar(t *testing.T) {
	user := newTestUser()

	avatar, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/"+uuid.NewV4().String(), true)
	require.NoError(t, err)
	user.SetAvatar(avatar)

	sch := UserToSchema(user)
	assert.NotEmpty(t, sch.Avatar, "avatar column stores the encoded form")
	assert.NotContains(t, sch.Avatar, "users/avatar/", "the encoded form must not contain the plain path")
	assert.Empty(t, sch.PasswordHash, "UserToSchema never writes a password")

	back, err := UserFromSchema(sch)
	require.NoError(t, err)

	assert.Equal(t, user.Id, back.Id)
	assert.Equal(t, user.Email, back.Email)
	assert.Equal(t, user.EmailVerifiedAt, back.EmailVerifiedAt)
	assert.Equal(t, user.Nickname, back.Nickname)
	assert.Equal(t, avatar, back.Avatar)
	assert.True(t, back.Avatar.IsPublic)
}

func TestUserToSchemaWithPassword(t *testing.T) {
	user := newTestUser()

	sch := UserToSchemaWithPassword(user, "bcrypt-hash")
	assert.Equal(t, "bcrypt-hash", sch.PasswordHash)
	assert.Equal(t, "users_v2", sch.TableName())

	// 口令不会从 schema 回读进实体
	back, err := UserFromSchema(sch)
	require.NoError(t, err)
	assert.Equal(t, user.Id, back.Id)
}

func TestUserRoundTrip_WithoutAvatar(t *testing.T) {
	user := newTestUser()

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
	user := newTestUser()

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
	assert.Equal(t, user.EmailVerifiedAt, back.EmailVerifiedAt)
	assert.Equal(t, user.Nickname, back.Nickname)
	assert.Equal(t, user.Status, back.Status)
	assert.Equal(t, avatar, back.Avatar)
	assert.True(t, back.Avatar.IsPublic)
	assert.Equal(t, user.CreatedAt.Value(), back.CreatedAt.Value())
	assert.Equal(t, user.UpdatedAt.Value(), back.UpdatedAt.Value())
}

func TestUserCacheSchemaRoundTrip_WithoutAvatar(t *testing.T) {
	user := newTestUser()

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

func TestUserIdentityRoundTrip(t *testing.T) {
	userId := valobj.NewUid()
	identity := identityentity.NewUserIdentity(userId, identityentity.ProviderTypeGithub, "sub-1", "Nick@Example.com", true)

	sch := UserIdentityToSchema(identity)
	assert.Equal(t, "user_identities_v2", sch.TableName())
	assert.Equal(t, "nick@example.com", sch.ProviderEmail, "provider email snapshot is normalized")

	back := UserIdentityFromSchema(sch)
	assert.Equal(t, identity.Id, back.Id)
	assert.Equal(t, userId, back.UserId)
	assert.Equal(t, identity.Provider, back.Provider)
	assert.Equal(t, identity.Subject, back.Subject)
	assert.Equal(t, identity.ProviderEmailVerified, back.ProviderEmailVerified)
	assert.Equal(t, identity.CreatedAt.Value(), back.CreatedAt.Value())

	assert.True(t, back.BelongsTo(userId))
}

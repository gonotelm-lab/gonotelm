package user

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUserRepo struct {
	user *identityentity.User
	err  error
}

func (r *fakeUserRepo) Save(context.Context, *identityentity.User) error { return nil }
func (r *fakeUserRepo) GetById(context.Context, valobj.Uid) (*identityentity.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.user, nil
}
func (r *fakeUserRepo) GetByProviderSub(context.Context, identityentity.ProviderType, string) (*identityentity.User, error) {
	return nil, identityerrors.ErrUserNotFound
}

type fakePublicURLer struct {
	url  string
	err  error
	keys []valobj.StoreKey
}

func (f *fakePublicURLer) PublicURL(_ context.Context, key valobj.StoreKey) (string, error) {
	f.keys = append(f.keys, key)
	if f.err != nil {
		return "", f.err
	}
	return f.url, nil
}

func meContext(userId valobj.Uid) context.Context {
	return pkgcontext.WithUserId(context.Background(), userId)
}

func TestGetMe_ReturnsAvatarPublicURL(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
	avatar, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/abc", true)
	require.NoError(t, err)
	user.SetAvatar(avatar)

	pub := &fakePublicURLer{url: "https://cdn.example.com/gonotelm-public/users/avatar/abc"}
	h := NewGetMeHandler(&fakeUserRepo{user: user}, pub)

	result, err := h.Handle(meContext(user.Id))
	require.NoError(t, err)

	assert.Equal(t, user.Id.String(), result.UserId)
	assert.Equal(t, "nick", result.Nickname)
	assert.Equal(t, "https://cdn.example.com/gonotelm-public/users/avatar/abc", result.AvatarUrl)
	assert.Equal(t, []valobj.StoreKey{avatar}, pub.keys)
}

func TestGetMe_NoAvatarReturnsEmptyURL(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
	pub := &fakePublicURLer{url: "https://cdn.example.com/should-not-be-used"}
	h := NewGetMeHandler(&fakeUserRepo{user: user}, pub)

	result, err := h.Handle(meContext(user.Id))
	require.NoError(t, err)

	assert.Empty(t, result.AvatarUrl)
	assert.Empty(t, pub.keys, "无头像时不该去解析 URL")
}

func TestGetMe_AvatarURLFailureDegradesToEmpty(t *testing.T) {
	user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
	avatar, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/abc", true)
	require.NoError(t, err)
	user.SetAvatar(avatar)

	h := NewGetMeHandler(&fakeUserRepo{user: user}, &fakePublicURLer{err: assert.AnError})

	result, err := h.Handle(meContext(user.Id))
	require.NoError(t, err, "头像解析失败不该让 /user/me 挂掉")

	assert.Equal(t, "nick", result.Nickname)
	assert.Empty(t, result.AvatarUrl)
}

func TestGetMe_UserLookupFailurePropagates(t *testing.T) {
	h := NewGetMeHandler(&fakeUserRepo{err: identityerrors.ErrUserNotFound}, &fakePublicURLer{})

	_, err := h.Handle(meContext(valobj.NewUid()))
	assert.ErrorIs(t, err, identityerrors.ErrUserNotFound)
}

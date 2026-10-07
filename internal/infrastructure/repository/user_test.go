package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	dbschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeUserStore struct {
	rows map[valobj.Uid]*dbschema.User

	byIdCalls     []valobj.Uid
	byProviderSub [][2]string
	upserts       []*dbschema.User
}

var _ interface {
	Create(ctx context.Context, user *dbschema.User) error
	Upsert(ctx context.Context, user *dbschema.User) error
	GetById(ctx context.Context, id valobj.Uid) (*dbschema.User, error)
	GetByProviderAndSub(ctx context.Context, provider, sub string) (*dbschema.User, error)
} = &fakeUserStore{}

func newFakeUserStore(rows ...*dbschema.User) *fakeUserStore {
	store := &fakeUserStore{rows: map[valobj.Uid]*dbschema.User{}}
	for _, row := range rows {
		store.rows[row.Id] = row
	}

	return store
}

func (s *fakeUserStore) Create(_ context.Context, user *dbschema.User) error {
	s.rows[user.Id] = user

	return nil
}

func (s *fakeUserStore) Upsert(_ context.Context, user *dbschema.User) error {
	s.upserts = append(s.upserts, user)
	s.rows[user.Id] = user

	return nil
}

func (s *fakeUserStore) GetById(_ context.Context, id valobj.Uid) (*dbschema.User, error) {
	s.byIdCalls = append(s.byIdCalls, id)

	row, ok := s.rows[id]
	if !ok {
		return nil, pkgerrors.ErrNoRecord
	}

	return row, nil
}

func (s *fakeUserStore) GetByProviderAndSub(
	_ context.Context, provider, sub string,
) (*dbschema.User, error) {
	s.byProviderSub = append(s.byProviderSub, [2]string{provider, sub})

	for _, row := range s.rows {
		if row.Provider == provider && row.Sub == sub {
			return row, nil
		}
	}

	return nil, pkgerrors.ErrNoRecord
}

type fakeUserCache struct {
	items  map[string]*cacheschema.User
	getErr error
	setErr error
}

var _ interface {
	Set(ctx context.Context, user *cacheschema.User) error
	GetById(ctx context.Context, id string) (*cacheschema.User, error)
} = &fakeUserCache{}

func newFakeUserCache() *fakeUserCache {
	return &fakeUserCache{items: map[string]*cacheschema.User{}}
}

func (c *fakeUserCache) Set(_ context.Context, user *cacheschema.User) error {
	if c.setErr != nil {
		return c.setErr
	}
	c.items[user.Id] = user

	return nil
}

func (c *fakeUserCache) GetById(_ context.Context, id string) (*cacheschema.User, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	return c.items[id], nil
}

func userRow(id valobj.Uid, provider, sub string) *dbschema.User {
	return &dbschema.User{
		Id:        id,
		Email:     "nick@example.com",
		Nickname:  "nick",
		Status:    string(identityentity.UserStatusActive),
		Provider:  provider,
		Sub:       sub,
		CreatedAt: 1700000000000,
		UpdatedAt: 1700000000001,
	}
}

func TestUserRepositoryGetByIdCacheAside(t *testing.T) {
	Convey("GetById backfills the cache on a miss and skips the store afterwards", t, func() {
		ctx := context.Background()
		id := valobj.NewUid()
		store := newFakeUserStore(userRow(id, string(identityentity.ProviderTypeGithub), "sub-1"))
		userCache := newFakeUserCache()
		repo := NewUserRepository(store, userCache)

		got, err := repo.GetById(ctx, id)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, id)
		So(store.byIdCalls, ShouldHaveLength, 1)
		So(userCache.items, ShouldContainKey, id.String())

		got, err = repo.GetById(ctx, id)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, id)
		So(store.byIdCalls, ShouldHaveLength, 1)
	})
}

func TestUserRepositoryGetByProviderSubBypassesCache(t *testing.T) {
	Convey("GetByProviderSub always reads the store and never touches the cache", t, func() {
		ctx := context.Background()
		id := valobj.NewUid()
		store := newFakeUserStore(userRow(id, string(identityentity.ProviderTypeGithub), "sub-1"))
		userCache := newFakeUserCache()
		repo := NewUserRepository(store, userCache)

		got, err := repo.GetByProviderSub(ctx, identityentity.ProviderTypeGithub, "sub-1")
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, id)
		So(store.byProviderSub, ShouldHaveLength, 1)
		So(userCache.items, ShouldBeEmpty)
		So(store.byIdCalls, ShouldBeEmpty)

		_, err = repo.GetByProviderSub(ctx, identityentity.ProviderTypeGithub, "sub-1")
		So(err, ShouldBeNil)
		So(store.byProviderSub, ShouldHaveLength, 2)
	})
}

func TestUserRepositoryGetNotFound(t *testing.T) {
	Convey("A missing user returns ErrUserNotFound and is not cached", t, func() {
		ctx := context.Background()
		store := newFakeUserStore()
		userCache := newFakeUserCache()
		repo := NewUserRepository(store, userCache)

		got, err := repo.GetById(ctx, valobj.NewUid())
		So(got, ShouldBeNil)
		So(errors.Is(err, identityerrors.ErrUserNotFound), ShouldBeTrue)

		got, err = repo.GetByProviderSub(ctx, identityentity.ProviderTypeGoogle, "missing")
		So(got, ShouldBeNil)
		So(errors.Is(err, identityerrors.ErrUserNotFound), ShouldBeTrue)
		So(userCache.items, ShouldBeEmpty)
	})
}

func TestUserRepositoryCacheFailureFallsBack(t *testing.T) {
	Convey("A cache read failure degrades to the store without failing the call", t, func() {
		ctx := context.Background()
		id := valobj.NewUid()
		store := newFakeUserStore(userRow(id, string(identityentity.ProviderTypeGithub), "sub-1"))
		userCache := newFakeUserCache()
		userCache.getErr = pkgerrors.ErrCache
		repo := NewUserRepository(store, userCache)

		got, err := repo.GetById(ctx, id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(store.byIdCalls, ShouldHaveLength, 1)
	})
}

func TestUserRepositoryCorruptCacheFallsBack(t *testing.T) {
	Convey("A corrupt cache entry degrades to the store", t, func() {
		ctx := context.Background()
		id := valobj.NewUid()
		store := newFakeUserStore(userRow(id, string(identityentity.ProviderTypeGithub), "sub-1"))
		userCache := newFakeUserCache()
		userCache.items[id.String()] = &cacheschema.User{Id: "not-a-ulid"}
		repo := NewUserRepository(store, userCache)

		got, err := repo.GetById(ctx, id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(store.byIdCalls, ShouldHaveLength, 1)
	})
}

func TestUserRepositorySaveRefreshesCache(t *testing.T) {
	Convey("Save writes through the cache so later reads skip the store", t, func() {
		ctx := context.Background()
		store := newFakeUserStore()
		userCache := newFakeUserCache()
		repo := NewUserRepository(store, userCache)

		user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
		So(repo.Save(ctx, user), ShouldBeNil)
		So(store.upserts, ShouldHaveLength, 1)

		user.Ban()
		So(repo.Save(ctx, user), ShouldBeNil)

		got, err := repo.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.IsBanned(), ShouldBeTrue)
		So(store.byIdCalls, ShouldBeEmpty)
	})
}

func TestUserRepositorySaveCacheFailureStillSucceeds(t *testing.T) {
	Convey("A cache write failure does not fail an already persisted Save", t, func() {
		ctx := context.Background()
		store := newFakeUserStore()
		userCache := newFakeUserCache()
		userCache.setErr = pkgerrors.ErrCache
		repo := NewUserRepository(store, userCache)

		user := identityentity.NewUser("nick", identityentity.ProviderTypeGithub, "sub-1")
		So(repo.Save(ctx, user), ShouldBeNil)
		So(store.upserts, ShouldHaveLength, 1)
	})
}

func TestUserRepositoryWithoutCache(t *testing.T) {
	Convey("Without a cache the repository degrades to the plain store", t, func() {
		ctx := context.Background()
		id := valobj.NewUid()
		store := newFakeUserStore(userRow(id, string(identityentity.ProviderTypeGithub), "sub-1"))
		repo := NewUserRepository(store, nil)

		got, err := repo.GetById(ctx, id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)

		got, err = repo.GetByProviderSub(ctx, identityentity.ProviderTypeGithub, "sub-1")
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)

		So(repo.Save(ctx, got), ShouldBeNil)
		So(store.upserts, ShouldHaveLength, 1)
	})
}

package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newTestOAuthState() *schema.OAuthState {
	return &schema.OAuthState{
		State:               "state-" + uuid.NewV7().String(),
		Nonce:               "nonce-" + uuid.NewV7().String(),
		CodeVerifier:        "verifier-" + uuid.NewV7().String(),
		CodeChallenge:       "challenge-" + uuid.NewV7().String(),
		CodeChallengeMethod: "S256",
		ReturnTo:            "https://example.com/return",
		ProviderType:        "github",
		Device:              "web",
	}
}

func TestOAuthStateCacheImpl_SetGetDelete(t *testing.T) {
	Convey("OAuthStateCache Set/Get/Delete", t, func() {
		ctx := t.Context()
		state := "state-" + uuid.NewV7().String()
		oauthState := newTestOAuthState()
		oauthState.State = state

		err := testOAuthStateCache.Set(ctx, state, oauthState)
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, state)

		got, err := testOAuthStateCache.Get(ctx, state)
		So(err, ShouldBeNil)
		So(got.State, ShouldEqual, oauthState.State)
		So(got.Nonce, ShouldEqual, oauthState.Nonce)
		So(got.CodeVerifier, ShouldEqual, oauthState.CodeVerifier)
		So(got.CodeChallenge, ShouldEqual, oauthState.CodeChallenge)
		So(got.CodeChallengeMethod, ShouldEqual, oauthState.CodeChallengeMethod)
		So(got.ReturnTo, ShouldEqual, oauthState.ReturnTo)
		So(got.ProviderType, ShouldEqual, oauthState.ProviderType)
		So(got.Device, ShouldEqual, oauthState.Device)

		err = testOAuthStateCache.Delete(ctx, state)
		So(err, ShouldBeNil)

		_, err = testOAuthStateCache.Get(ctx, state)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestOAuthStateCacheImpl_KeyIsHashed(t *testing.T) {
	Convey("OAuthStateCache key is hashed", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testOAuthStateCache.(*OAuthStateCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testOAuthStateCache.Set(ctx, state, newTestOAuthState())
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, state)

		// state 本身不能出现在 redis key 里，只能是它的 sha256 十六进制摘要
		hashed := cacheImpl.sha256State(state)
		So(cacheImpl.key(state), ShouldEqual, "gonotelm:identity:oauth_state:"+hashed)
		So(len(hashed), ShouldEqual, 64)

		exists, err := testRedis.Exists(ctx, cacheImpl.key(state)).Result()
		So(err, ShouldBeNil)
		So(exists, ShouldEqual, 1)
	})
}

func TestOAuthStateCacheImpl_StateIsolation(t *testing.T) {
	Convey("OAuthStateCache isolates states", t, func() {
		ctx := t.Context()
		stateA := "state-a-" + uuid.NewV7().String()
		stateB := "state-b-" + uuid.NewV7().String()

		infoA := newTestOAuthState()
		infoA.Nonce = "nonce-a"
		infoB := newTestOAuthState()
		infoB.Nonce = "nonce-b"

		err := testOAuthStateCache.Set(ctx, stateA, infoA)
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, stateA)

		err = testOAuthStateCache.Set(ctx, stateB, infoB)
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, stateB)

		gotA, err := testOAuthStateCache.Get(ctx, stateA)
		So(err, ShouldBeNil)
		So(gotA.Nonce, ShouldEqual, "nonce-a")

		gotB, err := testOAuthStateCache.Get(ctx, stateB)
		So(err, ShouldBeNil)
		So(gotB.Nonce, ShouldEqual, "nonce-b")
	})
}

func TestOAuthStateCacheImpl_NotFound(t *testing.T) {
	Convey("OAuthStateCache get missing state", t, func() {
		_, err := testOAuthStateCache.Get(t.Context(), "missing-state-"+uuid.NewV7().String())
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestOAuthStateCacheImpl_DeleteMissingIsNoop(t *testing.T) {
	Convey("OAuthStateCache delete missing state is noop", t, func() {
		err := testOAuthStateCache.Delete(t.Context(), "missing-state-"+uuid.NewV7().String())
		So(err, ShouldBeNil)
	})
}

func TestOAuthStateCacheImpl_Overwrite(t *testing.T) {
	Convey("OAuthStateCache overwrites existing state", t, func() {
		ctx := t.Context()
		state := "state-" + uuid.NewV7().String()

		first := newTestOAuthState()
		first.Nonce = "first"
		err := testOAuthStateCache.Set(ctx, state, first)
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, state)

		second := newTestOAuthState()
		second.Nonce = "second"
		err = testOAuthStateCache.Set(ctx, state, second)
		So(err, ShouldBeNil)

		got, err := testOAuthStateCache.Get(ctx, state)
		So(err, ShouldBeNil)
		So(got.Nonce, ShouldEqual, "second")
	})
}

func TestOAuthStateCacheImpl_TTL(t *testing.T) {
	Convey("OAuthStateCache applies oauthStateTTL", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testOAuthStateCache.(*OAuthStateCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testOAuthStateCache.Set(ctx, state, newTestOAuthState())
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, state)

		ttl, err := testRedis.TTL(ctx, cacheImpl.key(state)).Result()
		So(err, ShouldBeNil)
		So(ttl > 0, ShouldBeTrue)
		So(ttl <= oauthStateTTL, ShouldBeTrue)
		So(oauthStateTTL, ShouldEqual, 600*time.Second)
	})
}

func TestOAuthStateCacheImpl_CorruptedPayload(t *testing.T) {
	Convey("OAuthStateCache rejects corrupted payload", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testOAuthStateCache.(*OAuthStateCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testOAuthStateCache.Set(ctx, state, newTestOAuthState())
		So(err, ShouldBeNil)
		defer testOAuthStateCache.Delete(ctx, state)

		// 写入合法数据后手工篡改 payload，读取时必须报 ErrSerde，不返回脏数据
		err = testRedis.Set(ctx, cacheImpl.key(state), []byte{0xc1}, time.Minute).Err()
		So(err, ShouldBeNil)

		_, err = testOAuthStateCache.Get(ctx, state)
		So(errors.Is(err, errors.ErrSerde), ShouldBeTrue)
	})
}

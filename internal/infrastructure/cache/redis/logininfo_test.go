package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newTestLoginInfo() *schema.TransientProviderLoginInfo {
	return &schema.TransientProviderLoginInfo{
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

func TestTransientProviderLoginInfoCacheImpl_SetGetDelete(t *testing.T) {
	Convey("LoginInfoCache Set/Get/Delete", t, func() {
		ctx := t.Context()
		state := "state-" + uuid.NewV7().String()
		loginInfo := newTestLoginInfo()
		loginInfo.State = state

		err := testLoginInfoCache.Set(ctx, state, loginInfo)
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, state)

		got, err := testLoginInfoCache.Get(ctx, state)
		So(err, ShouldBeNil)
		So(got.State, ShouldEqual, loginInfo.State)
		So(got.Nonce, ShouldEqual, loginInfo.Nonce)
		So(got.CodeVerifier, ShouldEqual, loginInfo.CodeVerifier)
		So(got.CodeChallenge, ShouldEqual, loginInfo.CodeChallenge)
		So(got.CodeChallengeMethod, ShouldEqual, loginInfo.CodeChallengeMethod)
		So(got.ReturnTo, ShouldEqual, loginInfo.ReturnTo)
		So(got.ProviderType, ShouldEqual, loginInfo.ProviderType)
		So(got.Device, ShouldEqual, loginInfo.Device)

		err = testLoginInfoCache.Delete(ctx, state)
		So(err, ShouldBeNil)

		_, err = testLoginInfoCache.Get(ctx, state)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestTransientProviderLoginInfoCacheImpl_KeyIsHashed(t *testing.T) {
	Convey("LoginInfoCache key is hashed", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testLoginInfoCache.(*TransientProviderLoginInfoCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testLoginInfoCache.Set(ctx, state, newTestLoginInfo())
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, state)

		// state 明文不能出现在 redis key 里，只能出现其 sha256 十六进制摘要。
		hashed := cacheImpl.sha256State(state)
		So(cacheImpl.key(state), ShouldEqual, "gonotelm:login_info:state:"+hashed)
		So(len(hashed), ShouldEqual, 64)

		exists, err := testRedis.Exists(ctx, cacheImpl.key(state)).Result()
		So(err, ShouldBeNil)
		So(exists, ShouldEqual, 1)
	})
}

func TestTransientProviderLoginInfoCacheImpl_StateIsolation(t *testing.T) {
	Convey("LoginInfoCache isolates states", t, func() {
		ctx := t.Context()
		stateA := "state-a-" + uuid.NewV7().String()
		stateB := "state-b-" + uuid.NewV7().String()

		infoA := newTestLoginInfo()
		infoA.Nonce = "nonce-a"
		infoB := newTestLoginInfo()
		infoB.Nonce = "nonce-b"

		err := testLoginInfoCache.Set(ctx, stateA, infoA)
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, stateA)

		err = testLoginInfoCache.Set(ctx, stateB, infoB)
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, stateB)

		gotA, err := testLoginInfoCache.Get(ctx, stateA)
		So(err, ShouldBeNil)
		So(gotA.Nonce, ShouldEqual, "nonce-a")

		gotB, err := testLoginInfoCache.Get(ctx, stateB)
		So(err, ShouldBeNil)
		So(gotB.Nonce, ShouldEqual, "nonce-b")
	})
}

func TestTransientProviderLoginInfoCacheImpl_NotFound(t *testing.T) {
	Convey("LoginInfoCache get missing state", t, func() {
		_, err := testLoginInfoCache.Get(t.Context(), "missing-state-"+uuid.NewV7().String())
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestTransientProviderLoginInfoCacheImpl_DeleteMissingIsNoop(t *testing.T) {
	Convey("LoginInfoCache delete missing state is noop", t, func() {
		err := testLoginInfoCache.Delete(t.Context(), "missing-state-"+uuid.NewV7().String())
		So(err, ShouldBeNil)
	})
}

func TestTransientProviderLoginInfoCacheImpl_Overwrite(t *testing.T) {
	Convey("LoginInfoCache overwrites existing state", t, func() {
		ctx := t.Context()
		state := "state-" + uuid.NewV7().String()

		first := newTestLoginInfo()
		first.Nonce = "first"
		err := testLoginInfoCache.Set(ctx, state, first)
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, state)

		second := newTestLoginInfo()
		second.Nonce = "second"
		err = testLoginInfoCache.Set(ctx, state, second)
		So(err, ShouldBeNil)

		got, err := testLoginInfoCache.Get(ctx, state)
		So(err, ShouldBeNil)
		So(got.Nonce, ShouldEqual, "second")
	})
}

func TestTransientProviderLoginInfoCacheImpl_TTL(t *testing.T) {
	Convey("LoginInfoCache applies loginInfoTTL", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testLoginInfoCache.(*TransientProviderLoginInfoCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testLoginInfoCache.Set(ctx, state, newTestLoginInfo())
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, state)

		ttl, err := testRedis.TTL(ctx, cacheImpl.key(state)).Result()
		So(err, ShouldBeNil)
		So(ttl > 0, ShouldBeTrue)
		So(ttl <= loginInfoTTL, ShouldBeTrue)
		So(loginInfoTTL, ShouldEqual, 600*time.Second)
	})
}

func TestTransientProviderLoginInfoCacheImpl_CorruptedPayload(t *testing.T) {
	Convey("LoginInfoCache rejects corrupted payload", t, func() {
		ctx := t.Context()
		cacheImpl, ok := testLoginInfoCache.(*TransientProviderLoginInfoCacheImpl)
		So(ok, ShouldBeTrue)

		state := "state-" + uuid.NewV7().String()
		err := testLoginInfoCache.Set(ctx, state, newTestLoginInfo())
		So(err, ShouldBeNil)
		defer testLoginInfoCache.Delete(ctx, state)

		// 写入合法数据后手工篡改 payload，读取时必须报 ErrSerde 而不是返回脏数据。
		err = testRedis.Set(ctx, cacheImpl.key(state), []byte{0xc1}, time.Minute).Err()
		So(err, ShouldBeNil)

		_, err = testLoginInfoCache.Get(ctx, state)
		So(errors.Is(err, errors.ErrSerde), ShouldBeTrue)
	})
}

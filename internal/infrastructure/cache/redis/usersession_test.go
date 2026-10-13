package redis

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	goredis "github.com/redis/go-redis/v9"
	. "github.com/smartystreets/goconvey/convey"
)

// pipelineFailClient lets every other command through, only pipeline Exec fails
type pipelineFailClient struct {
	goredis.UniversalClient
}

func (c *pipelineFailClient) TxPipeline() goredis.Pipeliner {
	return &pipelineFailPipeliner{Pipeliner: c.UniversalClient.TxPipeline()}
}

type pipelineFailPipeliner struct {
	goredis.Pipeliner
}

func (p *pipelineFailPipeliner) Exec(context.Context) ([]goredis.Cmder, error) {
	return nil, stderrors.New("pipeline exec failed")
}

// 复用实现的 key 生成，避免测试硬编码
func sessionKeyOf(c cache.UserSessionCache, id string) string {
	impl, ok := c.(*UserSessionCacheImpl)
	if !ok {
		panic("unexpected user session cache impl")
	}
	return impl.sessionKey(id)
}

func userKeyOf(c cache.UserSessionCache, userId string) string {
	impl, ok := c.(*UserSessionCacheImpl)
	if !ok {
		panic("unexpected user session cache impl")
	}
	return impl.userKey(userId)
}

func TestUserSessionCacheImpl(t *testing.T) {
	Convey("UserSessionCache Set/Get/DeleteByUserId", t, func() {
		ctx := t.Context()
		id := "sess" + uuid.NewV7().String()
		userId := "user" + uuid.NewV7().String()

		session := &schema.UserSession{
			UserId:    userId,
			CreatedAt: time.Now().UnixMilli(),
			ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
			Device:    "web",
		}

		err := testUserSessionCache.Set(ctx, id, session, time.Hour)
		So(err, ShouldBeNil)

		got, err := testUserSessionCache.Get(ctx, id)
		So(err, ShouldBeNil)
		So(got.UserId, ShouldEqual, userId)
		So(got.Device, ShouldEqual, "web")

		err = testUserSessionCache.DeleteByUserId(ctx, userId)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestUserSessionCacheImplDelete(t *testing.T) {
	Convey("UserSessionCache Delete", t, func() {
		ctx := t.Context()
		id := "sess" + uuid.NewV7().String()
		userId := "user" + uuid.NewV7().String()

		session := &schema.UserSession{UserId: userId, CreatedAt: time.Now().UnixMilli(), ExpireAt: time.Now().Add(time.Hour).UnixMilli()}
		err := testUserSessionCache.Set(ctx, id, session, time.Hour)
		So(err, ShouldBeNil)

		err = testUserSessionCache.Delete(ctx, id)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestUserSessionCacheImplNotFound(t *testing.T) {
	Convey("UserSessionCache get missing session", t, func() {
		_, err := testUserSessionCache.Get(t.Context(), "missing"+uuid.NewV7().String())
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

// 一个账号多个会话，Delete 只删自己那份
func TestUserSessionCacheImplMultipleSessionsPerUser(t *testing.T) {
	Convey("UserSessionCache keeps multiple sessions per user", t, func() {
		ctx := t.Context()
		userId := "user" + uuid.NewV7().String()
		idA := "sess-a-" + uuid.NewV7().String()
		idB := "sess-b-" + uuid.NewV7().String()

		expireAt := time.Now().Add(time.Hour).UnixMilli()
		for _, id := range []string{idA, idB} {
			session := &schema.UserSession{
				UserId:    userId,
				CreatedAt: time.Now().UnixMilli(),
				ExpireAt:  expireAt,
				Device:    "web",
			}
			err := testUserSessionCache.Set(ctx, id, session, time.Hour)
			So(err, ShouldBeNil)
		}
		defer testUserSessionCache.DeleteByUserId(ctx, userId)

		gotB, err := testUserSessionCache.Get(ctx, idB)
		So(err, ShouldBeNil)
		So(gotB.UserId, ShouldEqual, userId)

		// 删掉 A 不应影响 B，user 集合里仍保留 B
		err = testUserSessionCache.Delete(ctx, idA)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, idA)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)

		_, err = testUserSessionCache.Get(ctx, idB)
		So(err, ShouldBeNil)

		// DeleteByUserId 清掉该账号剩余的会话
		err = testUserSessionCache.DeleteByUserId(ctx, userId)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, idB)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

// session 记录已过期或不存在时，Delete 仍要摘掉 user 集合成员并不报错
func TestUserSessionCacheImplDeleteWithoutRecord(t *testing.T) {
	Convey("UserSessionCache Delete without session record", t, func() {
		ctx := t.Context()
		userId := "user" + uuid.NewV7().String()
		id := "sess-" + uuid.NewV7().String()

		session := &schema.UserSession{
			UserId:    userId,
			CreatedAt: time.Now().UnixMilli(),
			ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
		}
		err := testUserSessionCache.Set(ctx, id, session, time.Hour)
		So(err, ShouldBeNil)
		defer testUserSessionCache.DeleteByUserId(ctx, userId)

		// 先删掉 session 记录，模拟 TTL 早于集合成员失效
		err = testRedis.Del(ctx, sessionKeyOf(testUserSessionCache, id)).Err()
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)

		err = testUserSessionCache.Delete(ctx, id)
		So(err, ShouldBeNil)
	})
}

func TestUserSessionCacheImplDeleteByUserIdWithoutSessions(t *testing.T) {
	Convey("UserSessionCache DeleteByUserId without sessions", t, func() {
		userId := "user" + uuid.NewV7().String()
		err := testUserSessionCache.DeleteByUserId(t.Context(), userId)
		So(err, ShouldBeNil)
	})
}

// Set 的 TTL 同时作用于 session 记录和 user 集合，避免集合成员永久残留
func TestUserSessionCacheImplSetTTL(t *testing.T) {
	Convey("UserSessionCache Set applies ttl to both keys", t, func() {
		ctx := t.Context()
		userId := "user" + uuid.NewV7().String()
		id := "sess-" + uuid.NewV7().String()

		session := &schema.UserSession{
			UserId:    userId,
			CreatedAt: time.Now().UnixMilli(),
			ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
		}
		err := testUserSessionCache.Set(ctx, id, session, time.Hour)
		So(err, ShouldBeNil)
		defer testUserSessionCache.DeleteByUserId(ctx, userId)

		sessionTTL, err := testRedis.TTL(ctx, sessionKeyOf(testUserSessionCache, id)).Result()
		So(err, ShouldBeNil)
		So(sessionTTL > 0, ShouldBeTrue)
		So(sessionTTL <= time.Hour, ShouldBeTrue)

		userTTL, err := testRedis.TTL(ctx, userKeyOf(testUserSessionCache, userId)).Result()
		So(err, ShouldBeNil)
		So(userTTL > 0, ShouldBeTrue)
		So(userTTL <= time.Hour, ShouldBeTrue)
	})
}

// DeleteByUserId 只清理目标账号，其他账号的会话不能误删
func TestUserSessionCacheImplDeleteByUserIdScopedToUser(t *testing.T) {
	Convey("UserSessionCache DeleteByUserId only targets one user", t, func() {
		ctx := t.Context()
		userId := "user" + uuid.NewV7().String()
		otherUserId := "user" + uuid.NewV7().String()
		id := "sess-" + uuid.NewV7().String()
		otherId := "sess-" + uuid.NewV7().String()

		expireAt := time.Now().Add(time.Hour).UnixMilli()
		err := testUserSessionCache.Set(ctx, id, &schema.UserSession{
			UserId: userId, CreatedAt: time.Now().UnixMilli(), ExpireAt: expireAt,
		}, time.Hour)
		So(err, ShouldBeNil)

		err = testUserSessionCache.Set(ctx, otherId, &schema.UserSession{
			UserId: otherUserId, CreatedAt: time.Now().UnixMilli(), ExpireAt: expireAt,
		}, time.Hour)
		So(err, ShouldBeNil)
		defer testUserSessionCache.DeleteByUserId(ctx, otherUserId)

		err = testUserSessionCache.DeleteByUserId(ctx, userId)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)

		_, err = testUserSessionCache.Get(ctx, otherId)
		So(err, ShouldBeNil)
	})
}

func TestUserSessionCacheImplCancelledContext(t *testing.T) {
	Convey("UserSessionCache wraps redis failures as ErrCache", t, func() {
		ctx := canceledCtx(t)
		userId := "user-cancelled-" + uuid.NewV7().String()
		id := "sess-cancelled-" + uuid.NewV7().String()
		session := &schema.UserSession{
			UserId:    userId,
			CreatedAt: time.Now().UnixMilli(),
			ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
		}

		So(errors.Is(testUserSessionCache.Set(ctx, id, session, time.Hour), errors.ErrCache), ShouldBeTrue)

		_, err := testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrCache), ShouldBeTrue)

		// Delete returns the Get error as-is when it is not ErrNoRecord
		So(errors.Is(testUserSessionCache.Delete(ctx, id), errors.ErrCache), ShouldBeTrue)

		So(errors.Is(testUserSessionCache.DeleteByUserId(ctx, userId), errors.ErrCache), ShouldBeTrue)
	})
}

func TestUserSessionCacheImplCorruptedPayload(t *testing.T) {
	Convey("UserSessionCache rejects a corrupted payload with ErrSerde", t, func() {
		ctx := t.Context()
		impl := testUserSessionCache.(*UserSessionCacheImpl)
		id := "sess-corrupted-" + uuid.NewV7().String()
		key := impl.sessionKey(id)
		t.Cleanup(func() { _ = testRedis.Del(ctx, key).Err() })

		So(testRedis.Set(ctx, key, []byte{0xc1}, time.Minute).Err(), ShouldBeNil)

		_, err := testUserSessionCache.Get(ctx, id)
		So(errors.Is(err, errors.ErrSerde), ShouldBeTrue)
	})
}

func TestUserSessionCacheImplPipelineErrors(t *testing.T) {
	Convey("UserSessionCache wraps pipeline exec failures as ErrCache", t, func() {
		ctx := t.Context()
		impl := NewUserSessionCacheImpl(&pipelineFailClient{UniversalClient: testRedis})

		userId := "user-pipe-" + uuid.NewV7().String()
		id := "sess-pipe-" + uuid.NewV7().String()
		session := &schema.UserSession{
			UserId:    userId,
			CreatedAt: time.Now().UnixMilli(),
			ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
		}

		// the record must exist for Delete, so write it with a working client first
		So(testUserSessionCache.Set(ctx, id, session, time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testUserSessionCache.DeleteByUserId(ctx, userId) })

		So(errors.Is(impl.Set(ctx, id, session, time.Hour), errors.ErrCache), ShouldBeTrue)

		// Delete: Get succeeds, the pipeline fails
		So(errors.Is(impl.Delete(ctx, id), errors.ErrCache), ShouldBeTrue)

		// DeleteByUserId: SMembers succeeds, the pipeline fails
		So(errors.Is(impl.DeleteByUserId(ctx, userId), errors.ErrCache), ShouldBeTrue)

		// nothing was actually written or deleted by the failing pipeline
		got, err := testUserSessionCache.Get(ctx, id)
		So(err, ShouldBeNil)
		So(got.UserId, ShouldEqual, userId)
	})
}

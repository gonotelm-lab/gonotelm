package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

// 直接复用实现里的 key 生成逻辑，避免测试里硬编码 key 格式。
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

// 一个用户可以有多个会话：Set 要往 user 集合里累加，Delete 只摘掉自己那一份。
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

		// 删掉 A 不应影响 B，且 user 集合里仍保留 B。
		err = testUserSessionCache.Delete(ctx, idA)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, idA)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)

		_, err = testUserSessionCache.Get(ctx, idB)
		So(err, ShouldBeNil)

		// DeleteByUserId 应把该用户剩余的会话全部清掉。
		err = testUserSessionCache.DeleteByUserId(ctx, userId)
		So(err, ShouldBeNil)

		_, err = testUserSessionCache.Get(ctx, idB)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

// session 记录已过期/不存在时，Delete 仍要摘掉 user 集合成员并不报错。
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

		// 直接把 session 记录删掉，模拟 TTL 先于集合成员失效。
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

// Set 的 TTL 同时作用于 session 记录和 user 集合，避免集合成员永久残留。
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

// DeleteByUserId 只清理目标用户，其他用户的会话不能被误删。
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

package cache

import (
	"os"
	"strings"
	"testing"

	redistestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/redis"
	. "github.com/smartystreets/goconvey/convey"
)

func TestInitRedis(t *testing.T) {
	addrs := os.Getenv(redistestsuite.EnvGonotelmTestRedisAddrs)
	if addrs == "" {
		t.Skipf("missing env %s", redistestsuite.EnvGonotelmTestRedisAddrs)
	}

	Convey("Init wires the global redis client and GetRedis returns a live one", t, func() {
		So(Init(&RedisCacheConfig{Addrs: strings.Split(addrs, ",")}), ShouldBeNil)

		client := GetRedis()
		So(client, ShouldNotBeNil)
		So(client.Ping(t.Context()).Err(), ShouldBeNil)
	})
}

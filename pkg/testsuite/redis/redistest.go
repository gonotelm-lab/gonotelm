package redis

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const EnvGonotelmTestRedisAddrs = "TEST_GONOTELM_REDIS_ADDRS"

// TestRedis manages a Redis client for integration tests.
type TestRedis struct {
	client goredis.UniversalClient
	addrs  []string
}

func NewTestRedis(addrs []string) (*TestRedis, error) {
	cleaned := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		cleaned = append(cleaned, addr)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("redis addrs is empty")
	}
	return &TestRedis{addrs: cleaned}, nil
}

func NewTestRedisFromEnv() (*TestRedis, error) {
	raw := strings.TrimSpace(os.Getenv(EnvGonotelmTestRedisAddrs))
	if raw == "" {
		return nil, fmt.Errorf("missing required env var: %s", EnvGonotelmTestRedisAddrs)
	}
	return NewTestRedis(strings.Split(raw, ","))
}

func (t *TestRedis) GetClient() goredis.UniversalClient {
	if t == nil {
		return nil
	}
	return t.client
}

func (t *TestRedis) Setup() error {
	if t == nil {
		return fmt.Errorf("test redis is nil")
	}
	if t.client != nil {
		return fmt.Errorf("test redis already setup")
	}

	client := goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs:                 t.addrs,
		ContextTimeoutEnabled: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("ping redis failed: %w", err)
	}

	t.client = client
	return nil
}

func (t *TestRedis) Cleanup() error {
	if t == nil || t.client == nil {
		return nil
	}
	err := t.client.Close()
	t.client = nil
	return err
}

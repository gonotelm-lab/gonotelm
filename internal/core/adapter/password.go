package adapter

import "context"

// PasswordHasher 密码的单向散列与校验。
type PasswordHasher interface {
	Hash(ctx context.Context, plain string) (string, error)
	// Verify 哈希不匹配时返回非 nil error。
	Verify(ctx context.Context, hash, plain string) error
}

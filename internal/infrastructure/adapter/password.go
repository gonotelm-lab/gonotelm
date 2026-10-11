package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type BcryptHasher struct{}

var _ adapter.PasswordHasher = (*BcryptHasher)(nil)

func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{}
}

func (h *BcryptHasher) Hash(_ context.Context, plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func (h *BcryptHasher) Verify(_ context.Context, hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

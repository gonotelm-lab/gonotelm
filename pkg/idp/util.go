package idp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func RandomString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gen rand string failed, %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

package string

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Random 生成 n 个字符的随机串，字符集是 URL 安全的 base64 字母表。
func Random(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}

	// base64 每 4 个字符承载 3 字节，多取 1 字节保证截断后长度足够
	buf := make([]byte, (n*3)/4+1)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random string failed, %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf)[:n], nil
}

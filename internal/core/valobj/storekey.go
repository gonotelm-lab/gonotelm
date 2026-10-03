package valobj

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7/pkg/s3utils"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 编码版本
const storeKeySchemeV1 = "stkv1"

const (
	storeKeyQueryPublic = "pub"
	storeKeyQueryTrue   = "1"
	storeKeyQueryFalse  = "0"
)

// StoreKey 是对象存储中一个对象的自包含引用。
type StoreKey struct {
	Bucket   string `json:"bucket"` // 由 StoreKeyFactory 补齐，业务侧不硬编码
	Key      string `json:"key"`
	IsPublic bool   `json:"is_public"`
}

// String 是日志形态，不要用于持久化。
func (k StoreKey) String() string {
	return fmt.Sprintf("bucket=%s,key=%s,is_public=%t", k.Bucket, k.Key, k.IsPublic)
}

func (k StoreKey) Valid() bool {
	if k.Bucket == "" || k.Key == "" || strings.HasPrefix(k.Key, "/") {
		return false
	}

	return s3utils.CheckValidBucketName(k.Bucket) == nil
}

// Encode 返回持久化/传输形态：base64url（无 padding）包裹的 URI
//
//	stkv1://<bucket>/<escaped path>?pub=0|1
func (k StoreKey) Encode() string {
	if !k.Valid() {
		return ""
	}

	pub := storeKeyQueryFalse
	if k.IsPublic {
		pub = storeKeyQueryTrue
	}

	payload := (&url.URL{
		Scheme:   storeKeySchemeV1,
		Host:     k.Bucket,
		Path:     k.Key,
		RawQuery: storeKeyQueryPublic + "=" + pub,
	}).String()

	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func DecodeStoreKey(s string) (StoreKey, error) {
	if s == "" {
		return StoreKey{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return StoreKey{}, errors.ErrParams.Msgf("storekey: base64 decode failed: %v", err)
	}

	u, err := url.Parse(string(raw))
	if err != nil {
		return StoreKey{}, errors.ErrParams.Msgf("storekey: uri parse failed: %v", err)
	}
	if u.Scheme != storeKeySchemeV1 {
		return StoreKey{}, errors.ErrParams.Msgf("storekey: unknown scheme %q", u.Scheme)
	}

	k := StoreKey{
		Bucket: u.Host,
		// Path 带 authority 分隔符，剥掉恰好一个 "/"
		Key: strings.TrimPrefix(u.Path, "/"),
	}

	switch pub := u.Query().Get(storeKeyQueryPublic); pub {
	case storeKeyQueryTrue:
		k.IsPublic = true
	case storeKeyQueryFalse:
	default:
		return StoreKey{}, errors.ErrParams.Msgf("storekey: bad %s=%q", storeKeyQueryPublic, pub)
	}

	if !k.Valid() {
		return StoreKey{}, errors.ErrParams.Msgf("storekey: malformed payload bucket=%q key=%q", k.Bucket, k.Key)
	}

	return k, nil
}

func NewStoreKey(bucket, key string, isPublic bool) (StoreKey, error) {
	k := StoreKey{Bucket: bucket, Key: key, IsPublic: isPublic}
	if !k.Valid() {
		return StoreKey{}, errors.ErrParams.Msgf("storekey: invalid bucket=%q key=%q", bucket, key)
	}

	return k, nil
}

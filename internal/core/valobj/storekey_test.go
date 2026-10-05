package valobj

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hostileObjectPaths 覆盖真实生成器会产出的、以及理论上会遇到的难缠路径。
var hostileObjectPaths = []string{
	"file/0192f0aa-1b2c-7d3e-8f90-abcdefabcdef/0192f0aa-1b2c-7d3e-8f90-abcdefabcdef.pdf",
	"parsed_file/0192f0aa-1b2c-7d3e-8f90-abcdefabcdef/0192f0aa-1b2c-7d3e-8f90-abcdefabcdef",
	"artifact/nb/id.100%done",
	"tmp/artifact/nb/artifact-id/audio/turn_000001.wav",
	"weird/a%2Fb",
	"sp ace/x",
	"q?x=1#frag",
	"plus+sign",
	"amp&sign",
	"eq=sign",
	"colon:key",
	"中文/路径.pdf",
	`quote"q`,
	"a//b",
	"~/tilde",
	"trailing/",
}

func TestStoreKey_EncodeDecodeRoundTrip(t *testing.T) {
	for _, path := range hostileObjectPaths {
		for _, isPublic := range []bool{false, true} {
			t.Run(path+"/public="+boolName(isPublic), func(t *testing.T) {
				want, err := NewStoreKey("gonotelm", path, isPublic)
				require.NoError(t, err)

				encoded := want.Encode()
				require.NotEmpty(t, encoded)

				got, err := DecodeStoreKey(encoded)
				require.NoError(t, err)
				assert.Equal(t, want, got)
			})
		}
	}
}

func TestStoreKey_ZeroValueIsAbsentObject(t *testing.T) {
	zero := StoreKey{}

	assert.False(t, zero.Valid())
	assert.Empty(t, zero.Encode(), "零值编码为空串")
	assert.Equal(t, "bucket=,key=,is_public=false", zero.String())

	got, err := DecodeStoreKey("")
	require.NoError(t, err)
	assert.Equal(t, zero, got)
}

func TestStoreKey_StringIsReadableAndDoesNotAffectEncode(t *testing.T) {
	key, err := NewStoreKey("gonotelm", "file/nb/src.pdf", false)
	require.NoError(t, err)

	assert.Equal(t, "bucket=gonotelm,key=file/nb/src.pdf,is_public=false", key.String())
	// String 与 Encode 是两种形态，不能互相顶替
	assert.NotEqual(t, key.String(), key.Encode())
	assert.NotContains(t, key.Encode(), "gonotelm", "编码形态是不透明的")
}

func TestDecodeStoreKey_FailureModes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"not base64", "!!!not base64!!!"},
		{"foreign scheme", base64.RawURLEncoding.EncodeToString([]byte("stkv2://bucket/key?pub=0"))},
		{"plain url without scheme", base64.RawURLEncoding.EncodeToString([]byte("bucket/key"))},
		{"missing bucket", base64.RawURLEncoding.EncodeToString([]byte("stkv1:///key?pub=0"))},
		{"missing path", base64.RawURLEncoding.EncodeToString([]byte("stkv1://bucket?pub=0"))},
		{"path is only slashes", base64.RawURLEncoding.EncodeToString([]byte("stkv1://bucket///?pub=0"))},
		{"bad pub value", base64.RawURLEncoding.EncodeToString([]byte("stkv1://bucket/key?pub=maybe"))},
		{"missing pub value", base64.RawURLEncoding.EncodeToString([]byte("stkv1://bucket/key?pub="))},
		{"missing pub param", base64.RawURLEncoding.EncodeToString([]byte("stkv1://bucket/key"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeStoreKey(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestNewStoreKey_RejectsInvalidBucket(t *testing.T) {
	// 与 minio-go 客户端保持一致（s3utils.CheckValidBucketName，非严格版）
	// 注意：它容忍 "_"、":" 和大写字母，所以那些不算非法
	for _, bucket := range []string{"", "a", "with/slash", "192.168.0.1", "a..b", ".leading", "trailing.", strings.Repeat("x", 64)} {
		_, err := NewStoreKey(bucket, "file/nb/src.pdf", false)
		assert.Error(t, err, "bucket=%q", bucket)
	}
}

func TestNewStoreKey_RejectsLeadingSlash(t *testing.T) {
	// 以 "/" 开头会在 url.URL.String() 往返中被 authority 分隔符吃掉
	_, err := NewStoreKey("gonotelm", "/file/nb/src.pdf", false)
	assert.Error(t, err)

	_, err = NewStoreKey("gonotelm", "", false)
	assert.Error(t, err)
}

func TestStoreKey_JSONRendersAsObject(t *testing.T) {
	// ADR 0001 的护栏：给 StoreKey 加上 MarshalJSON 或 MarshalText 会让这里失败
	key, err := NewStoreKey("gonotelm", "artifact/nb/a.png", false)
	require.NoError(t, err)

	encoded, err := sonic.Marshal(struct {
		StoreKey StoreKey `json:"store_key"`
	}{StoreKey: key})
	require.NoError(t, err)

	assert.JSONEq(t,
		`{"store_key":{"bucket":"gonotelm","key":"artifact/nb/a.png","is_public":false}}`,
		string(encoded))
}

func boolName(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

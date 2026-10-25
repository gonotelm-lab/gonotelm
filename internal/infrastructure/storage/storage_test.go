package storage

import (
	"testing"
	"time"
)

func TestMinioUploadPresignExpiryDefault(t *testing.T) {
	c := MinioConfig{}
	if got := c.uploadPresignExpiryOrDefault(); got != 15*time.Minute {
		t.Fatalf("default upload presign expiry = %v, want 15m", got)
	}

	c.PresignUploadExpiry = 5 * time.Minute
	if got := c.uploadPresignExpiryOrDefault(); got != 5*time.Minute {
		t.Fatalf("override upload presign expiry = %v, want 5m", got)
	}
}

func TestConfigUploadExpiryValidate(t *testing.T) {
	cfg := Config{Endpoint: "localhost:9000", AccessKey: "ak", SecretKey: "sk", Bucket: "bk"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.PresignExpiry != 15*time.Minute {
		t.Fatalf("PresignExpiry = %v, want 15m", cfg.PresignExpiry)
	}
	if cfg.PresignUploadExpiry != 15*time.Minute {
		t.Fatalf("PresignUploadExpiry = %v, want 15m", cfg.PresignUploadExpiry)
	}
}

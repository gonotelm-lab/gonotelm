package impl

import (
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
)

type Config struct {
	Type  Type        `toml:"type"`
	Minio MinioConfig `toml:"minio"`
}

type MinioConfig struct {
	Endpoint      string        `toml:"endpoint"`
	AccessKey     string        `toml:"accessKey"`
	SecretKey     string        `toml:"secretKey"`
	Bucket        string        `toml:"bucket"`
	Region        string        `toml:"region"`
	Secure        bool          `toml:"secure"`
	PresignExpiry time.Duration `toml:"presignExpiry"`
}

func (c *Config) Bucket() string {
	switch c.Type {
	case Minio:
		return c.Minio.Bucket
	default:
		return ""
	}
}

func (c *Config) ObjectStorageConfig() (*storage.Config, error) {
	switch c.Type {
	case Minio:
		presignExpiry := 15 * time.Minute
		if c.Minio.PresignExpiry != 0 {
			presignExpiry = c.Minio.PresignExpiry
		}

		return &storage.Config{
			Endpoint:      c.Minio.Endpoint,
			Region:        c.Minio.Region,
			Bucket:        c.Minio.Bucket,
			AccessKey:     c.Minio.AccessKey,
			SecretKey:     c.Minio.SecretKey,
			Secure:        c.Minio.Secure,
			PresignExpiry: presignExpiry,
		}, nil
	default:
		return nil, fmt.Errorf("storage type %q is not supported", c.Type)
	}
}

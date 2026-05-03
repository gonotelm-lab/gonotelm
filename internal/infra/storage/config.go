package storage

import (
	"errors"
	"time"
)

type Config struct {
	Endpoint  string `toml:"endpoint"   json:"endpoint"`
	Region    string `toml:"region"     json:"region"`
	Bucket    string `toml:"bucket"     json:"bucket"`
	AccessKey string `toml:"access_key" json:"access_key"`
	SecretKey string `toml:"secret_key" json:"secret_key"`
	Secure    bool   `toml:"secure"     json:"secure"`

	PresignExpiry time.Duration `toml:"presign_expiry" json:"presign_expiry"`

	Extra map[string]string `toml:"extra" json:"extra"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.AccessKey == "" {
		return errors.New("access_key is required")
	}
	if c.SecretKey == "" {
		return errors.New("secret_key is required")
	}
	if c.Bucket == "" {
		return errors.New("bucket is required")
	}

	if c.PresignExpiry <= 0 {
		c.PresignExpiry = 15 * time.Minute
	}

	return nil
}

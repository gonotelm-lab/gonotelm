package impl

import (
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl/minio"
)

type Type string

const (
	Minio Type = "minio"
)

func New(cfg *Config) (storage.Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config is nil")
	}

	storageCfg, err := cfg.ObjectStorageConfig()
	if err != nil {
		return nil, err
	}

	switch cfg.Type {
	case Minio:
		return minio.New(storageCfg)
	default:
		return nil, fmt.Errorf("impl type %s is not supported", cfg.Type)
	}
}

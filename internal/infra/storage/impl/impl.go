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

func New(implType Type, cfg *storage.Config) (storage.Storage, error) {
	switch implType {
	case Minio:
		return minio.New(cfg)
	default:
		return nil, fmt.Errorf("impl type %s is not supported", implType)
	}
}

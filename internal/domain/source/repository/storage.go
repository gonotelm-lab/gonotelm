package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PresignUploadResult struct {
	Method  string
	Url     string
	Forms   map[string]string
	Headers map[string]string
}

type PresignGetResult struct {
	Url string
}

type StorageRepository interface {
	FileObjectGetter
	FileObjectDeleter

	UploadObject(ctx context.Context, storeKey string, content []byte, contentType string) error
	PresignUpload(ctx context.Context, fileContent *entity.FileSourceContent) (*PresignUploadResult, error)
	PresignGet(ctx context.Context, storeKey string) (*PresignGetResult, error)
	CheckExist(ctx context.Context, storeKey string) (bool, error)
}

var ErrObjectNotFound = errors.ErrNoRecord.Msg("file object not found")

type FileObjectGetter interface {
	// 返回 ErrObjectNotFound 表示对象不存在
	// 一次性获取对象全部内容
	GetObject(ctx context.Context, storeKey string) ([]byte, int64, error)
}

type FileObjectDeleter interface {
	// 删除对象
	DeleteObject(ctx context.Context, storeKey string) error
}

package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListSpec struct {
	Offset int
	Limit  int
}

func (s *ListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}

	return nil
}

type Repository interface {
	Save(ctx context.Context, s *Source) error
	FindById(ctx context.Context, id valobj.Id) (*Source, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *ListSpec) ([]*Source, error)
}

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
	PresignUpload(ctx context.Context, fileContent *FileSourceContent) (*PresignUploadResult, error)
	PresignGet(ctx context.Context, storeKey string) (*PresignGetResult, error)
	CheckExist(ctx context.Context, storeKey string) (bool, error)
}

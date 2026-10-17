package cleanup

import (
	"context"
	stderrors "errors"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Sweeper 幂等清扫删除后残留的行 / 向量 / 对象；立即清理与延迟清理共用同一份逻辑。
type Sweeper struct {
	sourceRepo    sourcerepo.Repository
	notebookRepo  notebookrepo.Repository
	sourceDocRepo sourcerepo.SourceDocRepository
	objectStore   adapter.ObjectStore
}

func NewSweeper(
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	objectStore adapter.ObjectStore,
) *Sweeper {
	return &Sweeper{
		sourceRepo:    sourceRepo,
		notebookRepo:  notebookRepo,
		sourceDocRepo: sourceDocRepo,
		objectStore:   objectStore,
	}
}

// Sweep 执行一轮清扫；返回 error 表示外部依赖失败
func (s *Sweeper) Sweep(ctx context.Context, p *Payload) error {
	switch p.Kind {
	case KindSource:
		return s.SweepSource(ctx, p.NotebookId, p.SourceId, p.Keys)
	case KindNotebook:
		return s.SweepNotebook(ctx, p.NotebookId, p.Sources)
	default:
		return errors.ErrParams.Msgf("unknown cleanup kind: %s", p.Kind)
	}
}

func (s *Sweeper) SweepSource(
	ctx context.Context,
	notebookId, sourceId valobj.Id,
	keys []string,
) error {
	if _, err := s.sourceRepo.FindById(ctx, sourceId); err == nil {
		slog.InfoContext(ctx, "source still exists, skip cleanup",
			slog.String("source_id", sourceId.String()))

		return nil
	} else if !errors.Is(err, sourceerr.ErrSourceNotFound) {
		return errors.WithMessage(err, "check source exists failed")
	}

	return s.purge(ctx, notebookId, []SourceRef{{SourceId: sourceId, Keys: keys}})
}

func (s *Sweeper) SweepNotebook(
	ctx context.Context,
	notebookId valobj.Id,
	refs []SourceRef,
) error {
	exists, err := s.notebookRepo.Exists(ctx, notebookId)
	if err != nil {
		return errors.WithMessage(err, "check notebook exists failed")
	}
	if exists {
		slog.InfoContext(ctx, "notebook still exists, skip cleanup",
			slog.String("notebook_id", notebookId.String()))

		return nil
	}

	var errs []error
	if err := s.purge(ctx, notebookId, refs); err != nil {
		errs = append(errs, err)
	}

	// 兜底：删除过程中失败可能留下 source 行
	if err := s.sourceRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete sources by notebook failed"))
	}

	return stderrors.Join(errs...)
}

func (s *Sweeper) purge(ctx context.Context, notebookId valobj.Id, refs []SourceRef) error {
	if len(refs) == 0 {
		return nil
	}

	sourceIds := make([]valobj.Id, 0, len(refs))
	keys := make([]valobj.StoreKey, 0, len(refs)*2)
	for _, ref := range refs {
		sourceIds = append(sourceIds, ref.SourceId)

		for _, encoded := range ref.Keys {
			if key, ok := decodeObjectKey(ctx, ref.SourceId, encoded); ok {
				keys = append(keys, key)
			}
		}
	}

	// 向量和对象都尽力删：一边失败不影响另一边，失败的照样冒泡让 flow 重投
	var errs []error
	if err := s.sourceDocRepo.BatchDeleteBySourceId(ctx, notebookId, sourceIds); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete source docs failed"))
	}

	if err := s.objectStore.BatchDeleteObject(ctx, keys); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete objects failed"))
	}

	return stderrors.Join(errs...)
}

func decodeObjectKey(ctx context.Context, sourceId valobj.Id, encoded string) (valobj.StoreKey, bool) {
	if encoded == "" {
		return valobj.StoreKey{}, false
	}

	key, err := valobj.DecodeStoreKey(encoded)
	if err != nil || !key.Valid() {
		slog.WarnContext(ctx, "decode object key failed, skip",
			slog.String("source_id", sourceId.String()),
			slog.String("store_key", encoded),
			slog.Any("err", err),
		)

		return valobj.StoreKey{}, false
	}

	return key, true
}

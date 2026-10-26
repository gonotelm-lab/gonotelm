package cleanup

import (
	"context"
	stderrors "errors"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// sweeper 幂等清扫删除后残留的行 / 向量 / 对象；立即清理与延迟清理共用同一份逻辑。
type sweeper struct {
	sourceRepo    sourcerepo.Repository
	notebookRepo  notebookrepo.Repository
	artifactRepo  artifactrepo.Repository
	sourceDocRepo sourcerepo.SourceDocRepository
	pinRepo       pinrepo.Repository
	objectStore   adapter.ObjectStore
}

func newSweeper(
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
	artifactRepo artifactrepo.Repository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	pinRepo pinrepo.Repository,
	objectStore adapter.ObjectStore,
) *sweeper {
	return &sweeper{
		sourceRepo:    sourceRepo,
		notebookRepo:  notebookRepo,
		artifactRepo:  artifactRepo,
		sourceDocRepo: sourceDocRepo,
		pinRepo:       pinRepo,
		objectStore:   objectStore,
	}
}

// Sweep 执行一轮清扫；返回 error 表示外部依赖失败
func (s *sweeper) Sweep(ctx context.Context, p *Payload) error {
	slog.DebugContext(ctx, "sweep cleanup payload",
		slog.String("kind", p.Kind),
		slog.String("notebook_id", p.NotebookId.String()),
		slog.String("source_id", p.SourceId.String()),
		slog.String("artifact_id", p.ArtifactId.String()),
		slog.Int("source_refs", len(p.Sources)),
		slog.Int("artifact_refs", len(p.Artifacts)),
		slog.Int("keys", len(p.Keys)),
	)

	switch p.Kind {
	case KindSource:
		return s.SweepSource(ctx, p.NotebookId, p.SourceId, p.Keys)
	case KindNotebook:
		return s.SweepNotebook(ctx, p.NotebookId, p.Sources, p.Artifacts)
	case KindArtifact:
		return s.SweepArtifact(ctx, p.NotebookId, p.ArtifactId, p.Keys)
	case KindCover:
		return s.SweepNotebookObsoleteCover(ctx, p.NotebookId, p.Keys)
	default:
		return errors.ErrParams.Msgf("unknown cleanup kind: %s", p.Kind)
	}
}

func (s *sweeper) SweepSource(
	ctx context.Context,
	notebookId, sourceId valobj.Id,
	keys []string,
) error {
	slog.DebugContext(ctx, "sweep source",
		slog.String("notebook_id", notebookId.String()),
		slog.String("source_id", sourceId.String()),
		slog.Int("keys", len(keys)),
	)

	// 记录还在说明是误触发，跳过
	if _, err := s.sourceRepo.FindById(ctx, sourceId); err == nil {
		slog.InfoContext(ctx, "source still exists, skip cleanup",
			slog.String("source_id", sourceId.String()))

		return nil
	} else if !errors.Is(err, sourceerr.ErrSourceNotFound) {
		return errors.WithMessage(err, "check source exists failed")
	}

	return s.purgeSources(ctx, notebookId, []SourceRef{{SourceId: sourceId, Keys: keys}})
}

func (s *sweeper) SweepArtifact(
	ctx context.Context,
	notebookId, artifactId valobj.Id,
	keys []string,
) error {
	slog.DebugContext(ctx, "sweep artifact",
		slog.String("notebook_id", notebookId.String()),
		slog.String("artifact_id", artifactId.String()),
		slog.Int("keys", len(keys)),
	)

	// 记录还在说明是误触发，跳过
	if _, err := s.artifactRepo.FindById(ctx, artifactId); err == nil {
		slog.InfoContext(ctx, "artifact still exists, skip cleanup",
			slog.String("artifact_id", artifactId.String()))

		return nil
	} else if !errors.Is(err, artifacterrors.ErrArtifactNotFound) {
		return errors.WithMessage(err, "check artifact exists failed")
	}

	return s.purgeArtifacts(ctx, []ArtifactRef{{ArtifactId: artifactId, Keys: keys}})
}

func (s *sweeper) SweepDeletedNotebookSources(
	ctx context.Context,
	notebookId valobj.Id,
	sources []*sourceentity.Source,
) error {
	var errs []error
	if err := s.purgeSources(ctx, notebookId, collectSourceRefs(sources)); err != nil {
		errs = append(errs, err)
	}
	if err := s.sourceRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete sources by notebook failed"))
	}

	return stderrors.Join(errs...)
}

func (s *sweeper) SweepDeletedNotebookArtifacts(
	ctx context.Context,
	notebookId valobj.Id,
	artifacts []*artifactentity.Artifact,
) error {
	var errs []error
	if err := s.purgeArtifacts(ctx, collectArtifactRefs(artifacts)); err != nil {
		errs = append(errs, err)
	}
	if err := s.artifactRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete artifacts by notebook failed"))
	}

	return stderrors.Join(errs...)
}

// 清理notebook过期遗留为完成上传的封面对象
func (s *sweeper) SweepNotebookObsoleteCover(ctx context.Context, notebookId valobj.Id, keys []string) error {
	slog.DebugContext(ctx, "sweep cover",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("keys", len(keys)),
	)

	n, err := s.notebookRepo.FindById(ctx, notebookId)
	if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
		// 笔记本不存在 笔记本被删掉了 所以需要清理keys 此处keys为悬垂对象了
		keysToDelete := make([]valobj.StoreKey, 0, len(keys))
		for _, encoded := range keys {
			if key, ok := decodeObjectKey(ctx, notebookId, encoded); ok {
				keysToDelete = append(keysToDelete, key)
			}
		}

		if err := s.objectStore.BatchDeleteObject(ctx, keysToDelete); err != nil {
			return errors.WithMessage(err, "delete cover objects failed")
		}

		return nil
	}
	if err != nil {
		return errors.WithMessage(err, "check notebook exists failed")
	}

	if len(keys) == 0 {
		slog.InfoContext(ctx, "cover cleanup payload without keys, skip",
			slog.String("notebook_id", notebookId.String()))
		return nil
	}

	// key 无效说明载荷损坏，不能拿去匹配
	key, ok := decodeObjectKey(ctx, notebookId, keys[0])
	if !ok {
		return nil
	}

	if !n.PendingCover.Valid() || key != n.PendingCover {
		// 如果在途的封面key过期 或者在封面上传过程中 key发生了变化(一般不会发生) 就跳过
		slog.InfoContext(ctx, "pending cover mismatched, skip cleanup",
			slog.String("notebook_id", notebookId.String()),
			slog.String("pending", n.PendingCover.String()),
			slog.String("payload_key", key.String()),
		)
		return nil
	}

	if err := s.objectStore.BatchDeleteObject(ctx, []valobj.StoreKey{key}); err != nil {
		return errors.WithMessage(err, "delete cover object failed")
	}

	n.ClearPendingCover()
	if err := s.notebookRepo.Save(ctx, n); err != nil {
		return errors.WithMessage(err, "save notebook after cover cleanup failed")
	}

	return nil
}

func (s *sweeper) SweepNotebook(ctx context.Context,
	notebookId valobj.Id,
	sources []SourceRef,
	artifacts []ArtifactRef,
) error {
	slog.DebugContext(ctx, "sweep notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_refs", len(sources)),
		slog.Int("artifact_refs", len(artifacts)),
	)

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
	if err := s.purgeSources(ctx, notebookId, sources); err != nil {
		errs = append(errs, err)
	}
	if err := s.purgeArtifacts(ctx, artifacts); err != nil {
		errs = append(errs, err)
	}

	// 兜底：删除过程中失败可能留下 source / artifact / pin 记录
	if err := s.sourceRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete sources by notebook failed"))
	}
	if err := s.artifactRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete artifacts by notebook failed"))
	}
	if s.pinRepo != nil {
		if err := s.pinRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
			errs = append(errs, errors.WithMessage(err, "delete pins by notebook failed"))
		}
	}

	return stderrors.Join(errs...)
}

func (s *sweeper) purgeSources(ctx context.Context, notebookId valobj.Id, refs []SourceRef) error {
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

	slog.DebugContext(ctx, "purge source refs",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_count", len(sourceIds)),
		slog.Int("object_count", len(keys)),
	)

	var errs []error
	if err := s.sourceDocRepo.BatchDeleteBySourceId(ctx, notebookId, sourceIds); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete source docs failed"))
	}

	if err := s.objectStore.BatchDeleteObject(ctx, keys); err != nil {
		errs = append(errs, errors.WithMessage(err, "delete objects failed"))
	}

	return stderrors.Join(errs...)
}

func (s *sweeper) purgeArtifacts(ctx context.Context, refs []ArtifactRef) error {
	if len(refs) == 0 {
		return nil
	}

	keys := make([]valobj.StoreKey, 0, len(refs))
	for _, ref := range refs {
		for _, encoded := range ref.Keys {
			if key, ok := decodeObjectKey(ctx, ref.ArtifactId, encoded); ok {
				keys = append(keys, key)
			}
		}
	}

	slog.DebugContext(ctx, "purge artifact objects",
		slog.Int("artifact_count", len(refs)),
		slog.Int("object_count", len(keys)),
	)

	if err := s.objectStore.BatchDeleteObject(ctx, keys); err != nil {
		return errors.WithMessage(err, "delete artifact objects failed")
	}

	return nil
}

func decodeObjectKey(ctx context.Context, ownerId valobj.Id, encoded string) (valobj.StoreKey, bool) {
	if encoded == "" {
		return valobj.StoreKey{}, false
	}

	key, err := valobj.DecodeStoreKey(encoded)
	if err != nil || !key.Valid() {
		slog.WarnContext(ctx, "decode object key failed, skip",
			slog.String("owner_id", ownerId.String()),
			slog.String("store_key", encoded),
			slog.Any("err", err),
		)

		return valobj.StoreKey{}, false
	}

	return key, true
}

func collectSourceRefs(sources []*sourceentity.Source) []SourceRef {
	refs := make([]SourceRef, 0, len(sources))

	for _, source := range sources {
		refs = append(refs, SourceRef{
			SourceId: source.Id,
			Keys:     EncodeKeys(source.ObjectStoreKeys()),
		})
	}

	return refs
}

func collectArtifactRefs(artifacts []*artifactentity.Artifact) []ArtifactRef {
	refs := make([]ArtifactRef, 0, len(artifacts))

	for _, artifact := range artifacts {
		keys := artifact.ObjectStoreKeys()
		if len(keys) == 0 {
			continue
		}

		refs = append(refs, ArtifactRef{
			ArtifactId: artifact.Id,
			Keys:       EncodeKeys(keys),
		})
	}

	return refs
}

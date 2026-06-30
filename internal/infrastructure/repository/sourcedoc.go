package repository

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"

	vdal "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema/mapper"
)

type SourceDocRepositoryImpl struct {
	embedder embedding.Embedder
	vStore   vdal.SourceDocStore
	cfg      SourceDocRepositoryConfig
}

type SourceDocRepositoryConfig struct {
	EmbedBatchSize      int
	EmbedMaxConcurrency int
}

func NewSourceDocRepository(
	embedder embedding.Embedder,
	vstore vdal.SourceDocStore,
	c SourceDocRepositoryConfig,
) repository.SourceDocRepository {
	return &SourceDocRepositoryImpl{embedder: embedder, vStore: vstore, cfg: c}
}

func (r *SourceDocRepositoryImpl) FindById(
	ctx context.Context,
	notebookId valobj.Id,
	sourceId valobj.Id,
	id string,
) (*entity.SourceDoc, error) {
	res, err := r.vStore.Get(ctx, &schema.SourceDocGetParams{
		NotebookId: notebookId.String(),
		SourceId:   sourceId.String(),
		DocId:      id,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get source doc failed")
	}

	return mapper.SchemaToSourceDoc(res)
}

func (r *SourceDocRepositoryImpl) BatchFindById(
	ctx context.Context,
	notebookId valobj.Id,
	sourceId valobj.Id,
	ids []string,
) ([]*entity.SourceDoc, error) {
	res, err := r.vStore.BatchGet(ctx, &schema.SourceDocBatchGetParams{
		NotebookId: notebookId.String(),
		SourceId:   sourceId.String(),
		DocIds:     ids,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "batch get source docs failed")
	}

	return mapper.SchemasToSourceDocs(res)
}

func (r *SourceDocRepositoryImpl) BatchSave(
	ctx context.Context,
	docs []*entity.SourceDoc,
) error {
	schemaDocs := make([]*schema.SourceDoc, 0, len(docs))
	for _, doc := range docs {
		schemaDocs = append(schemaDocs, mapper.SourceDocToSchema(doc))
	}
	if len(schemaDocs) == 0 {
		return nil
	}

	texts := make([]string, len(schemaDocs))
	for i, doc := range schemaDocs {
		texts[i] = doc.Content
	}

	batchSize, maxConcurrency := resolveEmbedBatchSettings(len(texts), r.cfg)
	embeddings, err := batch.ParallelMap(
		ctx,
		texts,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batchTexts []string) ([][]float64, error) {
			return r.embedder.EmbedStrings(ctx, batchTexts)
		},
	)
	if err != nil {
		return errors.WithMessage(err, "embed source docs failed")
	}
	if len(embeddings) != len(texts) {
		return errors.Wrapf(
			errors.ErrSerde,
			"embed result count mismatch, expected=%d, actual=%d",
			len(texts),
			len(embeddings),
		)
	}

	for i, doc := range schemaDocs {
		doc.Embedding = slices.CastFloat[float64, float32](embeddings[i])
	}

	if err := r.vStore.BatchInsert(ctx, schemaDocs); err != nil {
		return errors.WithMessage(err, "batch insert source docs failed")
	}

	return nil
}

func (r *SourceDocRepositoryImpl) BatchDeleteBySourceId(
	ctx context.Context,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
) error {
	sourceIdsStr := make([]string, 0, len(sourceIds))
	for _, sourceId := range sourceIds {
		sourceIdsStr = append(sourceIdsStr, sourceId.String())
	}
	err := r.vStore.BatchDelete(ctx, &schema.SourceDocBatchDeleteParams{
		NotebookId: notebookId.String(),
		SourceId:   sourceIdsStr,
	})
	if err != nil {
		return errors.WithMessage(err, "batch delete source docs failed")
	}

	return nil
}

func (r *SourceDocRepositoryImpl) Query(
	ctx context.Context,
	query *repository.SourceDocQueryParams,
) ([]*entity.SourceDoc, error) {
	sourceIdsStr := make([]string, 0, len(query.SourceId))
	for _, sourceId := range query.SourceId {
		sourceIdsStr = append(sourceIdsStr, sourceId.String())
	}

	embeddings, err := r.embedder.EmbedStrings(ctx, []string{query.Target})
	if err != nil {
		return nil, errors.WithMessage(err, "embed target failed")
	}
	embedding := slices.CastFloat[float64, float32](embeddings[0])

	schemaDocs, err := r.vStore.Query(ctx, &schema.SourceDocQueryParams{
		NotebookId: query.NotebookId.String(),
		SourceIds:  sourceIdsStr,
		Embedding:  embedding,
		Target:     query.Target,
		Limit:      query.Limit,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "query source docs failed")
	}

	return mapper.SchemasToSourceDocs(schemaDocs)
}

func resolveEmbedBatchSettings(total int, cfg SourceDocRepositoryConfig) (batchSize, maxConcurrency int) {
	batchSize = cfg.EmbedBatchSize
	if batchSize <= 0 || batchSize > total {
		batchSize = total
	}
	if batchSize <= 0 {
		batchSize = 1
	}

	maxConcurrency = cfg.EmbedMaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	return batchSize, maxConcurrency
}

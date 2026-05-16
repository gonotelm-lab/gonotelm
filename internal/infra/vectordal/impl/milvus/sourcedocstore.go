package milvus

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"golang.org/x/sync/errgroup"
)

const (
	collectionName       = "source_docs"
	defaultSearchLimit   = 10
	maxSearchLimit       = 100
	defaultListBatchSize = 500
	maxListBatchSize     = 2000
	collectionLoadWait   = 30 * time.Second

	partitionCount  = 16 // DO NOT MODIFY THIS VALUE
	partitionPrefix = "_p"

	notebookIDTemplateKey = "notebook_id"
	sourceIDsTemplateKey  = "source_ids"
	sourceIDTemplateKey   = "source_id"
	docIDTemplateKey      = "doc_id"
)

type SourceDocStoreImpl struct {
	cli *milvusclient.Client
}

var _ vectordal.SourceDocStore = &SourceDocStoreImpl{}

func NewSourceDocStoreImpl(cli *milvusclient.Client) (*SourceDocStoreImpl, error) {
	store := &SourceDocStoreImpl{
		cli: cli,
	}
	if err := store.loadCollection(context.Background()); err != nil {
		return nil, fmt.Errorf(
			"ensure collection loaded failed, collection=%s: %w",
			collectionName,
			err,
		)
	}
	return store, nil
}

func (s *SourceDocStoreImpl) loadCollection(ctx context.Context) error {
	if s == nil || s.cli == nil {
		return fmt.Errorf("milvus client is nil")
	}

	loadCtx, cancel := context.WithTimeout(ctx, collectionLoadWait)
	defer cancel()

	loadOpt := milvusclient.NewGetLoadStateOption(collectionName)
	loadState, err := s.cli.GetLoadState(loadCtx, loadOpt)
	if err != nil {
		return fmt.Errorf("get collection load state failed: %w", err)
	}
	if loadState.State == entity.LoadStateLoaded {
		return nil
	}

	loadCollectionOpt := milvusclient.NewLoadCollectionOption(collectionName)
	task, err := s.cli.LoadCollection(loadCtx, loadCollectionOpt)
	if err != nil {
		return fmt.Errorf("load collection failed: %w", err)
	}
	if err = task.Await(loadCtx); err != nil {
		return fmt.Errorf("await collection load failed: %w", err)
	}

	return nil
}

func (s *SourceDocStoreImpl) BatchInsert(
	ctx context.Context,
	docs []*schema.SourceDoc,
) error {
	if len(docs) == 0 {
		return nil
	}

	sourceMappings := make(map[string][]*schema.SourceDoc, len(docs))
	for _, doc := range docs {
		notebookID := strings.TrimSpace(doc.NotebookId)
		if notebookID == "" {
			return errors.ErrParams.Msg("notebook id is empty in source doc")
		}
		sourceMappings[notebookID] = append(sourceMappings[notebookID], doc)
	}

	// 每个notebookId有对应的partition
	notebookIdSet := make(map[string]struct{}, len(sourceMappings))
	// notebookId -> partitionName
	notebookIdPartitionMappings := make(map[string]string, len(notebookIdSet))
	for notebookID := range sourceMappings {
		partitionName := partitionNameByNotebookID(notebookID)
		notebookIdPartitionMappings[notebookID] = partitionName
		notebookIdSet[notebookID] = struct{}{}
	}

	// 每个partition分别插入
	// partitionName -> sourceDocs
	sourceDocsByPartition := make(map[string][]*schema.SourceDoc, len(notebookIdSet))
	for notebookID, partitionName := range notebookIdPartitionMappings {
		sourceDocsByPartition[partitionName] = append(sourceDocsByPartition[partitionName], sourceMappings[notebookID]...)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for partitionName, sourceDocs := range sourceDocsByPartition {
		eg.Go(func() error {
			var gErr error
			defer func() {
				if e := recover(); e != nil {
					gErr = fmt.Errorf("panic in safe do: %v", e)
				}
			}()

			rows := make([]any, 0, len(sourceDocs)) // []map[string]any
			for _, doc := range sourceDocs {
				rows = append(rows, doc.AsMap())
			}

			opt := milvusclient.NewRowBasedInsertOption(collectionName, rows...)
			opt.WithPartition(partitionName) // 不能链式使用 因为WithPartition返回colBase 导致后续类型不匹配
			result, err := s.cli.Upsert(ctx, opt)
			if err != nil {
				gErr = errors.Wrapf(err, "upsert source docs to milvus failed, partition=%s", partitionName)
			}
			if result.UpsertCount != int64(len(sourceDocs)) {
				// log only
				slog.WarnContext(ctx,
					"upsert source docs count mismatch",
					slog.String("partition", partitionName),
					slog.Int("expected", len(sourceDocs)),
					slog.Int("actual", int(result.UpsertCount)),
				)
			}

			return gErr
		})
	}

	err := eg.Wait()
	if err != nil {
		return errors.Wrap(err, "batch insert source docs failed")
	}

	return nil
}

func (s *SourceDocStoreImpl) BatchDelete(
	ctx context.Context,
	params *schema.SourceDocBatchDeleteParams,
) error {
	if params == nil {
		return errors.ErrParams.Msg("batch delete params is nil")
	}

	notebookID := strings.TrimSpace(params.NotebookId)
	if notebookID == "" {
		return errors.ErrParams.Msg("notebook id is empty")
	}

	partitionName := partitionNameByNotebookID(notebookID)
	sourceIDs := normalizeSourceIDs(params.SourceId)
	if len(sourceIDs) == 0 {
		// empty source id list means no-op for safety.
		return nil
	}

	for _, sourceID := range sourceIDs {
		expr := fmt.Sprintf(
			`%s == "%s" && %s == "%s"`,
			schema.FieldNotebookID,
			notebookID,
			schema.FieldSourceID,
			sourceID,
		)
		_, err := s.cli.Delete(
			ctx,
			milvusclient.NewDeleteOption(collectionName).
				WithPartition(partitionName).
				WithExpr(expr),
		)
		if err != nil {
			return errors.Wrapf(
				err,
				"batch delete source docs failed, notebook_id=%s, source_id=%s, partition=%s",
				notebookID,
				sourceID,
				partitionName,
			)
		}
	}

	return nil
}

func (s *SourceDocStoreImpl) Get(
	ctx context.Context,
	params *schema.SourceDocGetParams,
) (*schema.SourceDoc, error) {
	if params == nil {
		return nil, errors.ErrParams.Msg("get source doc params is nil")
	}

	notebookID := strings.TrimSpace(params.NotebookId)
	sourceID := strings.TrimSpace(params.SourceId)
	docID := strings.TrimSpace(params.DocId)
	if notebookID == "" {
		return nil, errors.ErrParams.Msg("notebook id is empty")
	}
	if sourceID == "" {
		return nil, errors.ErrParams.Msg("source id is empty")
	}
	if docID == "" {
		return nil, errors.ErrParams.Msg("doc id is empty")
	}

	partitionName := partitionNameByNotebookID(notebookID)
	filterExpr := fmt.Sprintf(
		`%s == {%s} && %s == {%s} && %s == {%s}`,
		schema.FieldNotebookID, notebookIDTemplateKey,
		schema.FieldSourceID, sourceIDTemplateKey,
		schema.FieldID, docIDTemplateKey,
	)

	opt := milvusclient.NewQueryOption(collectionName).
		WithPartitions(partitionName).
		WithLimit(1).
		WithOutputFields(
			schema.FieldID,
			schema.FieldNotebookID,
			schema.FieldSourceID,
			schema.FieldContent,
			schema.FieldOwner,
		).
		WithFilter(filterExpr).
		WithTemplateParam(notebookIDTemplateKey, notebookID).
		WithTemplateParam(sourceIDTemplateKey, sourceID).
		WithTemplateParam(docIDTemplateKey, docID)

	rs, err := s.cli.Query(ctx, opt)
	if err != nil {
		return nil, errors.WithMessage(err, "query source doc failed")
	}

	docs, err := extractSourceDocsFromResultSet(rs)
	if err != nil {
		return nil, errors.WithMessage(err, "decode source doc query result failed")
	}
	if len(docs) == 0 {
		return nil, errors.ErrNoRecord.Msg("source doc not found")
	}

	return docs[0], nil
}

func (s *SourceDocStoreImpl) Query(
	ctx context.Context,
	params *schema.SourceDocQueryParams,
) ([]*schema.SourceDoc, error) {
	if params == nil {
		return nil, errors.ErrParams.Msg("query params is nil")
	}

	notebookID := strings.TrimSpace(params.NotebookId)
	if notebookID == "" {
		return nil, errors.ErrParams.Msg("notebook id is empty")
	}
	sourceIDs := normalizeSourceIDs(params.SourceIds)
	target := strings.TrimSpace(params.Target)
	if target == "" {
		return nil, errors.ErrParams.Msg("target is empty")
	}
	hasEmbedding := len(params.Embedding) != 0

	searchLimit := resolveSearchLimit(params.Limit)

	partitionName := partitionNameByNotebookID(notebookID)
	filterExpr := buildQueryFilterExpr(len(sourceIDs) > 0)
	outputFields := []string{
		schema.FieldID,
		schema.FieldNotebookID,
		schema.FieldSourceID,
		schema.FieldContent,
		schema.FieldOwner,
	}

	var (
		resultSets []milvusclient.ResultSet
		err        error
	)
	applyFilter := func(req *milvusclient.AnnRequest) {
		req.WithFilter(filterExpr).WithTemplateParam(notebookIDTemplateKey, notebookID)
		if len(sourceIDs) > 0 {
			req.WithTemplateParam(sourceIDsTemplateKey, sourceIDs)
		}
	}

	switch {
	case hasEmbedding:
		denseReq := milvusclient.NewAnnRequest(
			schema.FieldEmbedding,
			searchLimit,
			entity.FloatVector(params.Embedding),
		)
		sparseReq := milvusclient.NewAnnRequest(
			schema.FieldSparseContent,
			searchLimit,
			entity.Text(target),
		)
		applyFilter(denseReq)
		applyFilter(sparseReq)
		opt := milvusclient.NewHybridSearchOption(
			collectionName,
			searchLimit,
			denseReq,
			sparseReq,
		).WithReranker(milvusclient.NewRRFReranker()).
			WithPartitions(partitionName).
			WithOutputFields(outputFields...)
		resultSets, err = s.cli.HybridSearch(ctx, opt)
		if err != nil {
			return nil, errors.WithMessage(err, "hybrid search source docs failed")
		}
	default:
		opt := milvusclient.NewSearchOption(
			collectionName,
			searchLimit,
			[]entity.Vector{entity.Text(target)},
		).WithANNSField(schema.FieldSparseContent).
			WithPartitions(partitionName).
			WithOutputFields(outputFields...)
		opt.WithFilter(filterExpr).WithTemplateParam(notebookIDTemplateKey, notebookID)
		if len(sourceIDs) > 0 {
			opt.WithTemplateParam(sourceIDsTemplateKey, sourceIDs)
		}
		resultSets, err = s.cli.Search(ctx, opt)
		if err != nil {
			return nil, errors.WithMessage(err, "bm25 sparse search source docs failed")
		}
	}

	docs, err := extractSourceDocsFromResults(resultSets)
	if err != nil {
		return nil, errors.WithMessage(err, "decode milvus query result failed")
	}

	return docs, nil
}

func (s *SourceDocStoreImpl) List(
	ctx context.Context,
	params *schema.SourceDocListParams,
) ([]*schema.SourceDoc, error) {
	if params == nil {
		return nil, errors.ErrParams.Msg("list params is nil")
	}

	notebookID := strings.TrimSpace(params.NotebookId)
	if notebookID == "" {
		return nil, errors.ErrParams.Msg("notebook id is empty")
	}

	sourceID := strings.TrimSpace(params.SourceId)
	if sourceID == "" {
		return nil, errors.ErrParams.Msg("source id is empty")
	}

	partitionName := partitionNameByNotebookID(notebookID)
	filterExpr := fmt.Sprintf(
		`%s == %s && %s == %s`,
		schema.FieldNotebookID, notebookID,
		schema.FieldSourceID, sourceID,
	)
	outputFields := []string{
		schema.FieldID,
		schema.FieldNotebookID,
		schema.FieldSourceID,
		schema.FieldContent,
		schema.FieldOwner,
	}
	batchSize := resolveListBatchSize(params.BatchSize)

	iter, err := s.cli.QueryIterator(
		ctx,
		milvusclient.NewQueryIteratorOption(collectionName).
			WithPartitions(partitionName).
			WithFilter(filterExpr).
			WithOutputFields(outputFields...).
			WithBatchSize(batchSize),
	)
	if err != nil {
		return nil, errors.WithMessage(err, "create source doc query iterator failed")
	}

	docs := make([]*schema.SourceDoc, 0, batchSize)
	for {
		rs, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.WithMessage(err, "iterate source docs failed")
		}

		batchDocs, err := extractSourceDocsFromResultSet(rs)
		if err != nil {
			return nil, errors.WithMessage(err, "decode source docs list batch failed")
		}
		docs = append(docs, batchDocs...)
	}

	return docs, nil
}

func partitionNameByNotebookID(notebookID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(notebookID))
	idx := h.Sum32() % partitionCount
	return fmt.Sprintf("%s%04d", partitionPrefix, idx)
}

func buildQueryFilterExpr(hasSourceIDs bool) string {
	expr := fmt.Sprintf(`%s == {%s}`, schema.FieldNotebookID, notebookIDTemplateKey)
	if hasSourceIDs {
		expr += fmt.Sprintf(` && %s in {%s}`, schema.FieldSourceID, sourceIDsTemplateKey)
	}
	return expr
}

func normalizeSourceIDs(sourceIDs []string) []string {
	if len(sourceIDs) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(sourceIDs))
	out := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		id := strings.TrimSpace(sourceID)
		if id == "" {
			continue
		}
		if _, ok := uniq[id]; ok {
			continue
		}
		uniq[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func resolveSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

func resolveListBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return defaultListBatchSize
	}
	if batchSize > maxListBatchSize {
		return maxListBatchSize
	}
	return batchSize
}

func extractSourceDocsFromResults(resultSets []milvusclient.ResultSet) ([]*schema.SourceDoc, error) {
	docs := make([]*schema.SourceDoc, 0, len(resultSets))
	for _, rs := range resultSets {
		setDocs, err := extractSourceDocsFromResultSet(rs)
		if err != nil {
			return nil, err
		}
		docs = append(docs, setDocs...)
	}

	return docs, nil
}

type sourceDocResultColumns struct {
	id         column.Column
	notebookID column.Column
	sourceID   column.Column
	content    column.Column
	owner      column.Column
}

func extractSourceDocsFromResultSet(rs milvusclient.ResultSet) ([]*schema.SourceDoc, error) {
	if rs.Err != nil {
		return nil, rs.Err
	}
	if rs.ResultCount == 0 {
		return nil, nil
	}

	cols, err := getSourceDocResultColumns(rs)
	if err != nil {
		return nil, err
	}

	docs := make([]*schema.SourceDoc, 0, rs.ResultCount)
	for i := 0; i < rs.ResultCount; i++ {
		doc, err := buildSourceDocFromColumns(cols, i)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func getSourceDocResultColumns(rs milvusclient.ResultSet) (*sourceDocResultColumns, error) {
	cols := &sourceDocResultColumns{
		id:         rs.GetColumn(schema.FieldID),
		notebookID: rs.GetColumn(schema.FieldNotebookID),
		sourceID:   rs.GetColumn(schema.FieldSourceID),
		content:    rs.GetColumn(schema.FieldContent),
		owner:      rs.GetColumn(schema.FieldOwner),
	}

	if missing := missingSourceDocColumns(cols); len(missing) > 0 {
		return nil, errors.Wrapf(
			errors.ErrSerde,
			"missing required fields in search result, fields=%v",
			missing,
		)
	}

	return cols, nil
}

func missingSourceDocColumns(cols *sourceDocResultColumns) []string {
	missing := make([]string, 0, 5)
	if cols.id == nil {
		missing = append(missing, schema.FieldID)
	}
	if cols.notebookID == nil {
		missing = append(missing, schema.FieldNotebookID)
	}
	if cols.sourceID == nil {
		missing = append(missing, schema.FieldSourceID)
	}
	if cols.content == nil {
		missing = append(missing, schema.FieldContent)
	}
	if cols.owner == nil {
		missing = append(missing, schema.FieldOwner)
	}
	return missing
}

func buildSourceDocFromColumns(cols *sourceDocResultColumns, index int) (*schema.SourceDoc, error) {
	id, err := cols.id.GetAsString(index)
	if err != nil {
		return nil, errors.WithMessage(err, "read id from result failed")
	}
	notebookID, err := cols.notebookID.GetAsString(index)
	if err != nil {
		return nil, errors.WithMessage(err, "read notebook_id from result failed")
	}
	sourceID, err := cols.sourceID.GetAsString(index)
	if err != nil {
		return nil, errors.WithMessage(err, "read source_id from result failed")
	}
	content, err := cols.content.GetAsString(index)
	if err != nil {
		return nil, errors.WithMessage(err, "read content from result failed")
	}
	owner, err := cols.owner.GetAsString(index)
	if err != nil {
		return nil, errors.WithMessage(err, "read owner from result failed")
	}

	return &schema.SourceDoc{
		Id:         id,
		NotebookId: notebookID,
		SourceId:   sourceID,
		Content:    content,
		Owner:      owner,
	}, nil
}

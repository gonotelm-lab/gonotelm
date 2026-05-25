package milvus

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"

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

var reservedSourceDocFields = slices.MapSet(schema.OutputFields)

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
					gErr = errors.Wrapf(errors.ErrInner, "panic in safe do: %v", e)
				}
			}()

			rows := make([]any, 0, len(sourceDocs)) // []map[string]any
			for _, doc := range sourceDocs {
				if doc == nil {
					continue
				}
				row := buildSourceDocMilvusRow(doc)
				rows = append(rows, row)
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
		return errors.WithMessage(err, "batch insert source docs failed")
	}

	return nil
}

func buildSourceDocMilvusRow(doc *schema.SourceDoc) map[string]any {
	row := doc.AsMap()
	for key, value := range doc.Meta {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		if _, ok := reservedSourceDocFields[k]; ok {
			continue
		}
		row[k] = value
	}

	return row
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
			`%s == %q && %s == %q`,
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
		WithOutputFields(schema.OutputFields...).
		WithFilter(filterExpr).
		WithTemplateParam(notebookIDTemplateKey, notebookID).
		WithTemplateParam(sourceIDTemplateKey, sourceID).
		WithTemplateParam(docIDTemplateKey, docID)

	rs, err := s.cli.Query(ctx, opt)
	if err != nil {
		return nil, errors.WithMessage(err, "query source doc failed")
	}

	docs, err := extractSourceDocsFromResultSet(ctx, rs)
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
			WithOutputFields(schema.OutputFields...)
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
			WithOutputFields(schema.OutputFields...)
		opt.WithFilter(filterExpr).WithTemplateParam(notebookIDTemplateKey, notebookID)
		if len(sourceIDs) > 0 {
			opt.WithTemplateParam(sourceIDsTemplateKey, sourceIDs)
		}
		resultSets, err = s.cli.Search(ctx, opt)
		if err != nil {
			return nil, errors.WithMessage(err, "bm25 sparse search source docs failed")
		}
	}

	docs, err := extractSourceDocsFromResults(ctx, resultSets)
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
		`%s == %q && %s == %q`,
		schema.FieldNotebookID, notebookID,
		schema.FieldSourceID, sourceID,
	)
	batchSize := resolveListBatchSize(params.BatchSize)

	iter, err := s.cli.QueryIterator(
		ctx,
		milvusclient.NewQueryIteratorOption(collectionName).
			WithPartitions(partitionName).
			WithFilter(filterExpr).
			WithOutputFields(schema.OutputFields...).
			WithBatchSize(batchSize),
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase,
			"create source doc query iterator failed, notebook_id=%q, source_id=%q, partition=%q, err=%v",
			notebookID, sourceID, partitionName, err,
		)
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

		batchDocs, err := extractSourceDocsFromResultSet(ctx, rs)
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

func extractSourceDocsFromResults(
	ctx context.Context,
	resultSets []milvusclient.ResultSet,
) ([]*schema.SourceDoc, error) {
	docs := make([]*schema.SourceDoc, 0, len(resultSets))
	for _, rs := range resultSets {
		setDocs, err := extractSourceDocsFromResultSet(ctx, rs)
		if err != nil {
			return nil, errors.WithMessage(err, "extract source docs from result set failed")
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
	chunkPos   column.Column
	meta       column.Column
	dynamic    []column.Column
}

func extractSourceDocsFromResultSet(
	ctx context.Context,
	rs milvusclient.ResultSet,
) ([]*schema.SourceDoc, error) {
	if rs.Err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "result set has upstream error, err=%v", rs.Err)
	}
	if rs.ResultCount == 0 {
		return nil, nil
	}

	cols, err := getSourceDocResultColumns(rs)
	if err != nil {
		return nil, errors.WithMessage(err, "get source doc result columns failed")
	}

	docs := make([]*schema.SourceDoc, 0, rs.ResultCount)
	for i := 0; i < rs.ResultCount; i++ {
		doc, err := buildSourceDocFromColumns(ctx, cols, i)
		if err != nil {
			return nil, errors.WithMessagef(err, "build source doc from columns failed, index=%d", i)
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
		chunkPos:   rs.GetColumn(schema.FieldChunkPos),
		meta:       rs.GetColumn(schema.FieldMeta),
	}
	for _, c := range rs.Fields {
		if c == nil {
			continue
		}
		fieldName := c.Name()
		if fieldName == "" {
			continue
		}
		if _, ok := reservedSourceDocFields[fieldName]; ok {
			continue
		}
		cols.dynamic = append(cols.dynamic, c)
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
	missing := make([]string, 0, 6)
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
	if cols.chunkPos == nil {
		missing = append(missing, schema.FieldChunkPos)
	}
	return missing
}

func buildSourceDocFromColumns(
	ctx context.Context,
	cols *sourceDocResultColumns,
	index int,
) (*schema.SourceDoc, error) {
	id, err := cols.id.GetAsString(index)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "read id from result failed, err=%v", err)
	}
	notebookID, err := cols.notebookID.GetAsString(index)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "read notebook_id from result failed, err=%v", err)
	}
	sourceID, err := cols.sourceID.GetAsString(index)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "read source_id from result failed, err=%v", err)
	}
	content, err := cols.content.GetAsString(index)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "read content from result failed, err=%v", err)
	}
	owner, err := cols.owner.GetAsString(index)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "read owner from result failed, err=%v", err)
	}
	chunkPos := int32(-1)
	if isNull, err := cols.chunkPos.IsNull(index); err != nil {
		// 兼容旧数据 chunk_pos可能为null 这里不处理
	} else if !isNull {
		chunkPosInt64, err := cols.chunkPos.GetAsInt64(index)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrDatabase, "read chunk_pos from result failed, err=%v", err)
		}
		chunkPos = int32(chunkPosInt64)
	}
	meta, err := buildSourceDocMeta(ctx, cols, index)
	if err != nil {
		slog.WarnContext(ctx,
			"build source doc meta failed, fallback without meta",
			slog.Int("index", index),
			slog.Any("err", err),
		)
		meta = nil
	}

	return &schema.SourceDoc{
		Id:         id,
		NotebookId: notebookID,
		SourceId:   sourceID,
		Content:    content,
		Owner:      owner,
		ChunkPos:   chunkPos,
		Meta:       meta,
	}, nil
}

func buildSourceDocMeta(
	ctx context.Context,
	cols *sourceDocResultColumns,
	index int,
) (map[string]any, error) {
	meta := make(map[string]any)

	if cols.meta != nil {
		if isNull, err := cols.meta.IsNull(index); err != nil {
			slog.WarnContext(ctx,
				"check $meta null state failed, skip meta",
				slog.Int("index", index),
				slog.Any("err", err),
			)
			return nil, nil
		} else if !isNull {
			rawMeta, err := cols.meta.Get(index)
			if err != nil {
				slog.WarnContext(ctx,
					"read $meta from result failed, skip meta",
					slog.Int("index", index),
					slog.Any("err", err),
				)
				return nil, nil
			}
			parsedMeta, err := parseMilvusMeta(rawMeta)
			if err != nil {
				slog.WarnContext(ctx,
					"parse $meta failed, skip meta",
					slog.Int("index", index),
					slog.Any("err", err),
				)
				return nil, nil
			}
			for key, value := range parsedMeta {
				if _, ok := reservedSourceDocFields[key]; ok {
					continue
				}
				meta[key] = value
			}
		}
	}

	for _, c := range cols.dynamic {
		if c == nil {
			continue
		}
		if isNull, err := c.IsNull(index); err != nil {
			slog.WarnContext(ctx,
				"check dynamic field null state failed, skip field",
				slog.Int("index", index),
				slog.String("field", c.Name()),
				slog.Any("err", err),
			)
			continue
		} else if isNull {
			continue
		}
		value, err := c.Get(index)
		if err != nil {
			slog.WarnContext(ctx,
				"read dynamic field from result failed, skip field",
				slog.Int("index", index),
				slog.String("field", c.Name()),
				slog.Any("err", err),
			)
			continue
		}
		meta[c.Name()] = value
	}

	if len(meta) == 0 {
		return nil, nil
	}
	return meta, nil
}

func parseMilvusMeta(raw any) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}

	switch val := raw.(type) {
	case map[string]any:
		return val, nil
	case []byte:
		return unmarshalMetaBytes(val)
	case string:
		return unmarshalMetaBytes([]byte(val))
	default:
		payload, err := json.Marshal(val)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrSerde, "marshal unknown $meta payload failed, err=%v", err)
		}
		return unmarshalMetaBytes(payload)
	}
}

func unmarshalMetaBytes(payload []byte) (map[string]any, error) {
	if len(payload) == 0 || string(payload) == "null" {
		return nil, nil
	}

	out := make(map[string]any)
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal $meta payload failed, err=%v", err)
	}
	return out, nil
}

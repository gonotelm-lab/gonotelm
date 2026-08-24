package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/clickhouse"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type EmbeddingLogStoreImpl struct {
	ch            *clickhouse.Conn
	createBatcher *clickhouse.BatchInserter
}

func NewEmbeddingLogStoreImpl(ctx context.Context, ch *clickhouse.Conn) (*EmbeddingLogStoreImpl, error) {
	impl := &EmbeddingLogStoreImpl{ch: ch}

	createBatcher, err := ch.CreateBatcher(ctx, fmt.Sprintf("INSERT INTO %s", schema.EmbeddingLog{}.TableName()))
	if err != nil {
		return nil, fmt.Errorf("can not create clickhouse create batcher: %w", err)
	}
	impl.createBatcher = createBatcher

	return impl, nil
}

var _ olap.EmbeddingLogStore = &EmbeddingLogStoreImpl{}

func (s *EmbeddingLogStoreImpl) Create(ctx context.Context, log *schema.EmbeddingLog) error {
	if log == nil {
		return nil
	}

	if log.ID == "" {
		log.ID = uuid.NewV7().String()
	}
	now := time.Now()
	if log.CreateTime.IsZero() {
		log.CreateTime = now
	}
	if log.UpdateTime.IsZero() {
		log.UpdateTime = now
	}

	err := s.createBatcher.Append(ctx, log)
	if err != nil {
		return errors.Wrapf(errors.ErrDatabase, "clickhouse append err=%v", err)
	}

	return nil
}

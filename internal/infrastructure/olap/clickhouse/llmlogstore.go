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

type LLMLogStoreImpl struct {
	ch            *clickhouse.Conn
	createBatcher *clickhouse.BatchInserter
}

func NewLLMLogStoreImpl(ctx context.Context,
	ch *clickhouse.Conn,
) (*LLMLogStoreImpl, error) {
	impl := &LLMLogStoreImpl{
		ch: ch,
	}
	createBatcher, err := ch.CreateBatcher(ctx, fmt.Sprintf("INSERT INTO %s", schema.LLMLog{}.TableName()))
	if err != nil {
		return nil, fmt.Errorf("can not create clickhouse create batcher: %w", err)
	}
	impl.createBatcher = createBatcher // clickhouse.conn is responsible for closing the batcher

	return impl, nil
}

var _ olap.LLMLogStore = &LLMLogStoreImpl{}

func (s *LLMLogStoreImpl) Create(ctx context.Context, log *schema.LLMLog) error {
	if log == nil {
		return nil
	}

	if log.Id == "" {
		log.Id = uuid.NewV7().String()
	}
	now := time.Now()
	if log.CreateTime.IsZero() {
		log.CreateTime = now
	}

	err := s.createBatcher.Append(ctx, log)
	if err != nil {
		return errors.Wrapf(errors.ErrDatabase, "clickhouse append err=%v", err)
	}

	return nil
}

func (s *LLMLogStoreImpl) Query(
	ctx context.Context,
	userId string,
	timeRange schema.TimeRange,
	extra *schema.ExtraQueryConditions,
) ([]*schema.LLMLog, error) {
	return selectLogsByUserTime[schema.LLMLog](
		ctx,
		s.ch,
		schema.LLMLogAllFields,
		schema.LLMLogTableName,
		userId,
		timeRange,
		extra,
	)
}

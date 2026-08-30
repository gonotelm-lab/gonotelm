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

type Text2AudioLogStoreImpl struct {
	ch            *clickhouse.Conn
	createBatcher *clickhouse.BatchInserter
}

func NewText2AudioLogStoreImpl(ctx context.Context, ch *clickhouse.Conn) (*Text2AudioLogStoreImpl, error) {
	impl := &Text2AudioLogStoreImpl{ch: ch}

	createBatcher, err := ch.CreateBatcher(ctx, fmt.Sprintf("INSERT INTO %s", schema.Text2AudioLog{}.TableName()))
	if err != nil {
		return nil, fmt.Errorf("can not create clickhouse create batcher: %w", err)
	}
	impl.createBatcher = createBatcher

	return impl, nil
}

var _ olap.Text2AudioLogStore = &Text2AudioLogStoreImpl{}

func (s *Text2AudioLogStoreImpl) Create(ctx context.Context, log *schema.Text2AudioLog) error {
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

func (s *Text2AudioLogStoreImpl) Query(
	ctx context.Context,
	userId string,
	timeRange schema.TimeRange,
	extra *schema.ExtraQueryConditions,
) ([]*schema.Text2AudioLog, error) {
	return selectLogsByUserTime[schema.Text2AudioLog](
		ctx,
		s.ch,
		schema.Text2AudioLogAllFields,
		schema.Text2AudioLogTableName,
		userId,
		timeRange,
		extra,
	)
}

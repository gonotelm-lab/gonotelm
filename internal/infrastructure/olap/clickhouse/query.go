package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/clickhouse"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func selectLogsByUserTime[T any](
	ctx context.Context,
	ch *clickhouse.Conn,
	fields string,
	table string,
	userId string,
	timeRange schema.TimeRange,
	extra *schema.ExtraQueryConditions,
) ([]*T, error) {
	sql := fmt.Sprintf(
		"SELECT %s FROM %s WHERE user_id = ? AND call_start_time >= ? AND call_start_time <= ? ",
		fields,
		table,
	)
	args := []any{userId, timeRange.Start, timeRange.End}
	var (
		sqlBuilder strings.Builder
		offset     = 0
		limit      = 50
	)
	factor := 1.0
	if extra != nil {
		factor = 1.2
	}
	sqlBuilder.Grow(int(float64(len(sql)) * factor))
	sqlBuilder.WriteString(sql)

	if extra != nil {
		if extra.GroupId != "" {
			sqlBuilder.WriteString(" AND group_id = ?")
			args = append(args, extra.GroupId)
		}
		if extra.TraceId != "" {
			sqlBuilder.WriteString(" AND trace_id = ?")
			args = append(args, extra.TraceId)
		}

		if extra.Offset >= 0 {
			offset = extra.Offset
		}
		if extra.Limit >= 0 {
			limit = extra.Limit
		}
	}

	sqlBuilder.WriteString(" ORDER BY call_start_time DESC LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	var rows []T
	if err := ch.Select(ctx, &rows, sqlBuilder.String(), args...); err != nil {
		return nil, errors.Wrapf(errors.ErrDatabase, "clickhouse select err=%v", err)
	}

	return valueSliceToPtrSlice(rows), nil
}

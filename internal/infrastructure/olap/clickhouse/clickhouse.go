package clickhouse

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/pkg/clickhouse"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

func Open(ctx context.Context, cfg *sql.Config) (*olap.Dao, error) {
	driver, err := ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: ch.Auth{
			Database: cfg.DBName,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Logger: clickhouse.DriverLogger(),
		Compression: &ch.Compression{
			Method: ch.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open failed: %w", err)
	}

	c := clickhouse.NewConn(driver)

	closer := misc.CloserFunc(func(ctx context.Context) error {
		return c.Close()
	})

	llmLogStore, err := NewLLMLogStoreImpl(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("create llm log store err: %w", err)
	}

	embeddingLogStore, err := NewEmbeddingLogStoreImpl(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("create embedding log store err: %w", err)
	}

	text2ImageLogStore, err := NewText2ImageLogStoreImpl(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("create text2image log store err: %w", err)
	}

	text2AudioLogStore, err := NewText2AudioLogStoreImpl(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("create text2audio log store err: %w", err)
	}

	return olap.NewDao(
		closer,
		llmLogStore,
		embeddingLogStore,
		text2ImageLogStore,
		text2AudioLogStore,
	), nil
}

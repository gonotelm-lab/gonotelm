package retry

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func Do(
	ctx context.Context,
	times int,
	cool time.Duration,
	fn func(ctx context.Context) error,
) error {
	var err error
	for i := 1; i <= times; i++ {
		err = fn(ctx)
		if err == nil {
			return nil
		}

		log := fmt.Sprintf("fn failed, retrying (%d/%d)", i, times)
		slog.WarnContext(ctx, log, slog.Any("err", err))
		// exponential backoff
		time.Sleep(cool * time.Duration(i))
	}

	return err
}

func DefaultDo(ctx context.Context, fn func(ctx context.Context) error) error {
	return Do(ctx, 3, 1*time.Second, fn)
}

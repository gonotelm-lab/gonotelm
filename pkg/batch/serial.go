package batch

import (
	"context"
	"fmt"
)

// BatchMap splits items into batches and processes them sequentially.
func BatchMap[In any, Out any](
	ctx context.Context,
	items []In,
	batchSize int,
	fn func(ctx context.Context, batch []In) ([]Out, error),
) ([]Out, error) {
	if fn == nil {
		return nil, fmt.Errorf("batch mapper is nil")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than 0")
	}
	if len(items) == 0 {
		return []Out{}, nil
	}

	results := make([]Out, 0, len(items))
	for start := 0; start < len(items); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}

		rows, err := fn(ctx, items[start:end])
		if err != nil {
			return nil, err
		}
		results = append(results, rows...)
	}

	return results, nil
}

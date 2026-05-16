package batch

import (
	"context"
	"fmt"
	"sync"
)

// ParallelMap splits items into batches and processes batches concurrently.
// The flattened output keeps the same batch order as the input.
func ParallelMap[In any, Out any](
	ctx context.Context,
	items []In,
	batchSize int,
	maxConcurrency int,
	fn func(ctx context.Context, batch []In) ([]Out, error),
) ([]Out, error) {
	if fn == nil {
		return nil, fmt.Errorf("batch mapper is nil")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than 0")
	}
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("max concurrency must be greater than 0")
	}
	if len(items) == 0 {
		return []Out{}, nil
	}

	batchCount := (len(items) + batchSize - 1) / batchSize
	batchResults := make([][]Out, batchCount)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, maxConcurrency)
	)
	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		mu.Unlock()
	}

	for i := range batchCount {
		start := i * batchSize
		end := min(start + batchSize, len(items))

		wg.Add(1)
		go func(batchIdx, from, to int) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-runCtx.Done():
				return
			}
			defer func() { <-sem }()

			result, err := fn(runCtx, items[from:to])
			if err != nil {
				setErr(err)
				return
			}

			batchResults[batchIdx] = result
		}(i, start, end)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	total := 0
	for _, part := range batchResults {
		total += len(part)
	}

	flatten := make([]Out, 0, total)
	for _, part := range batchResults {
		flatten = append(flatten, part...)
	}

	return flatten, nil
}

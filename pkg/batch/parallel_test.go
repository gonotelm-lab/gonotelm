package batch

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

func TestParallelMapKeepsOrder(t *testing.T) {
	input := []int{1, 2, 3, 4, 5, 6, 7}

	got, err := ParallelMap(context.Background(), input, 3, 2, func(ctx context.Context, batch []int) ([]int, error) {
		out := make([]int, len(batch))
		for i, v := range batch {
			out[i] = v * 10
		}
		return out, nil
	})
	if err != nil {
		t.Fatalf("parallel map failed: %v", err)
	}

	want := []int{10, 20, 30, 40, 50, 60, 70}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected output, got=%v want=%v", got, want)
	}
}

func TestParallelMapReturnsError(t *testing.T) {
	input := []int{1, 2, 3, 4}

	_, err := ParallelMap(context.Background(), input, 2, 2, func(ctx context.Context, batch []int) ([]int, error) {
		for _, v := range batch {
			if v == 3 {
				return nil, fmt.Errorf("boom")
			}
		}
		return batch, nil
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestParallelMapRejectsInvalidParams(t *testing.T) {
	input := []int{1}

	_, err := ParallelMap(context.Background(), input, 0, 1, func(ctx context.Context, batch []int) ([]int, error) {
		return batch, nil
	})
	if err == nil {
		t.Fatalf("expected batch size error, got nil")
	}

	_, err = ParallelMap(context.Background(), input, 1, 0, func(ctx context.Context, batch []int) ([]int, error) {
		return batch, nil
	})
	if err == nil {
		t.Fatalf("expected max concurrency error, got nil")
	}

	_, err = ParallelMap[int, int](context.Background(), input, 1, 1, nil)
	if err == nil {
		t.Fatalf("expected nil mapper error, got nil")
	}
}

package batch

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestBatchMapKeepsOrder(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}

	got, err := BatchMap(context.Background(), input, 2, func(ctx context.Context, batch []int) ([]int, error) {
		out := make([]int, len(batch))
		for i, v := range batch {
			out[i] = v * 10
		}
		return out, nil
	})
	if err != nil {
		t.Fatalf("batch map failed: %v", err)
	}

	want := []int{10, 20, 30, 40, 50}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected output, got=%v want=%v", got, want)
	}
}

func TestBatchMapReturnsError(t *testing.T) {
	input := []int{1, 2, 3}

	_, err := BatchMap(context.Background(), input, 2, func(ctx context.Context, batch []int) ([]int, error) {
		if batch[0] == 3 {
			return nil, errors.New("boom")
		}
		return batch, nil
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestBatchMapRejectsInvalidParams(t *testing.T) {
	input := []int{1}

	_, err := BatchMap(context.Background(), input, 0, func(ctx context.Context, batch []int) ([]int, error) {
		return batch, nil
	})
	if err == nil {
		t.Fatalf("expected batch size error, got nil")
	}

	_, err = BatchMap[int, int](context.Background(), input, 1, nil)
	if err == nil {
		t.Fatalf("expected nil mapper error, got nil")
	}
}

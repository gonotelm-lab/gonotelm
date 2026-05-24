package normalize

import (
	"math"
	"strings"
	"testing"
)

func TestL2(t *testing.T) {
	data := [][]float64{
		{3, 4},
		{0, 0},
	}

	normalized, err := L2(data)
	if err != nil {
		t.Fatalf("l2 normalize failed: %v", err)
	}

	if diff := math.Abs(normalized[0][0] - 0.6); diff > 1e-12 {
		t.Fatalf("unexpected normalized[0][0]: got=%v", normalized[0][0])
	}
	if diff := math.Abs(normalized[0][1] - 0.8); diff > 1e-12 {
		t.Fatalf("unexpected normalized[0][1]: got=%v", normalized[0][1])
	}
	if normalized[1][0] != 0 || normalized[1][1] != 0 {
		t.Fatalf("zero row should remain unchanged, got=%v", normalized[1])
	}
}

func TestL2WithEpsilonValidation(t *testing.T) {
	data := [][]float64{{1, 2}}
	_, err := L2WithEpsilon(data, -1)
	if err == nil {
		t.Fatalf("expected epsilon validation error, got nil")
	}
	if !strings.Contains(err.Error(), "epsilon must be finite and >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestZScore(t *testing.T) {
	data := [][]float64{
		{1, 10},
		{2, 10},
		{3, 10},
	}

	normalized, err := ZScore(data)
	if err != nil {
		t.Fatalf("zscore normalize failed: %v", err)
	}

	_, means, stds, err := ZScoreWithStats(data)
	if err != nil {
		t.Fatalf("zscore normalize with stats failed: %v", err)
	}
	if len(means) != 2 || len(stds) != 2 {
		t.Fatalf("unexpected parameter length: means=%d stds=%d", len(means), len(stds))
	}
	if diff := math.Abs(means[0] - 2); diff > 1e-12 {
		t.Fatalf("unexpected mean[0]: %v", means[0])
	}
	if stds[1] != 1 {
		t.Fatalf("constant column std should fallback to 1, got=%v", stds[1])
	}

	if math.Abs(normalized[1][0]) > 1e-12 {
		t.Fatalf("middle point should be 0 after zscore, got=%v", normalized[1][0])
	}
	if math.Abs(normalized[0][1]) > 1e-12 || math.Abs(normalized[2][1]) > 1e-12 {
		t.Fatalf("constant column should become zeros, got first=%v last=%v", normalized[0][1], normalized[2][1])
	}
}

func TestApplyZScoreValidation(t *testing.T) {
	data := [][]float64{{1, 2}}
	_, err := ApplyZScore(data, []float64{0}, []float64{1})
	if err == nil {
		t.Fatalf("expected parameter length mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "zscore parameter length mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

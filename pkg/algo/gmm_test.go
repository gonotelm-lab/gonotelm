package algo

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestGMMClusterTwoGroups(t *testing.T) {
	vectors := make([][]float64, 0, 80)
	for i := 0; i < 40; i++ {
		x := -3.0 + float64(i%8)*0.08 + float64(i/8)*0.02
		y := -2.8 + float64(i%5)*0.06 + float64(i/5)*0.02
		vectors = append(vectors, []float64{x, y})
	}
	for i := 0; i < 40; i++ {
		x := 4.2 + float64(i%8)*0.07 + float64(i/8)*0.03
		y := 4.0 + float64(i%5)*0.06 + float64(i/5)*0.02
		vectors = append(vectors, []float64{x, y})
	}

	result, err := GMMCluster(vectors, 2, &GMMOptions{
		MaxIterations:  120,
		Tolerance:      1e-8,
		Regularization: 1e-6,
		Seed:           7,
	})
	if err != nil {
		t.Fatalf("GMMCluster failed: %v", err)
	}

	if len(result.Labels) != len(vectors) {
		t.Fatalf("label size mismatch, got=%d want=%d", len(result.Labels), len(vectors))
	}
	if len(result.Responsibilities) != len(vectors) {
		t.Fatalf("responsibility row size mismatch, got=%d want=%d", len(result.Responsibilities), len(vectors))
	}
	if len(result.Means) != 2 {
		t.Fatalf("mean size mismatch, got=%d want=2", len(result.Means))
	}
	if len(result.Weights) != 2 {
		t.Fatalf("weight size mismatch, got=%d want=2", len(result.Weights))
	}
	if result.ClusterCount != 2 {
		t.Fatalf("unexpected cluster count, got=%d want=2", result.ClusterCount)
	}
	if result.Iterations <= 0 || result.Iterations > 120 {
		t.Fatalf("unexpected iteration count, got=%d", result.Iterations)
	}
	if math.IsNaN(result.LogLikelihood) || math.IsInf(result.LogLikelihood, 0) {
		t.Fatalf("invalid log likelihood: %v", result.LogLikelihood)
	}
	if math.IsNaN(result.BIC) || math.IsInf(result.BIC, 0) {
		t.Fatalf("invalid BIC: %v", result.BIC)
	}

	weightSum := 0.0
	for _, w := range result.Weights {
		weightSum += w
	}
	if math.Abs(weightSum-1) > 1e-6 {
		t.Fatalf("weights do not sum to 1, got=%v", weightSum)
	}

	for i := range result.Responsibilities {
		if len(result.Responsibilities[i]) != 2 {
			t.Fatalf("responsibility col size mismatch at row=%d", i)
		}
		rowSum := 0.0
		for _, p := range result.Responsibilities[i] {
			if p < 0 || p > 1 || math.IsNaN(p) || math.IsInf(p, 0) {
				t.Fatalf("invalid responsibility at row=%d value=%v", i, p)
			}
			rowSum += p
		}
		if math.Abs(rowSum-1) > 1e-6 {
			t.Fatalf("responsibility row does not sum to 1 at row=%d got=%v", i, rowSum)
		}
	}

	// 标签 0/1 的语义可能交换，取两种映射里更高的准确率。
	normal := 0
	reversed := 0
	for i, label := range result.Labels {
		trueLabel := 0
		if i >= 40 {
			trueLabel = 1
		}
		if label == trueLabel {
			normal++
		}
		if label == 1-trueLabel {
			reversed++
		}
	}
	accuracy := float64(maxInt(normal, reversed)) / float64(len(vectors))
	if accuracy < 0.95 {
		t.Fatalf("unexpected clustering accuracy, got=%0.4f want>=0.95", accuracy)
	}
}

func TestGMMClusterAutoSelectKByBIC(t *testing.T) {
	rng := rand.New(rand.NewSource(20260523))
	vectors := make([][]float64, 0, 240)
	for i := 0; i < 120; i++ {
		x := -3.5 + 0.45*rng.NormFloat64()
		y := -3.0 + 0.50*rng.NormFloat64()
		vectors = append(vectors, []float64{x, y})
	}
	for i := 0; i < 120; i++ {
		x := 3.8 + 0.40*rng.NormFloat64()
		y := 3.2 + 0.55*rng.NormFloat64()
		vectors = append(vectors, []float64{x, y})
	}

	result, err := GMMCluster(vectors, -2, &GMMOptions{
		MaxIterations:   150,
		Tolerance:       1e-8,
		Regularization:  1e-6,
		Seed:            11,
		AutoMaxClusters: 8,
	})
	if err != nil {
		t.Fatalf("GMMCluster auto-select failed: %v", err)
	}

	if result.ClusterCount != 2 {
		t.Fatalf("unexpected auto-selected cluster count, got=%d want=2", result.ClusterCount)
	}
	if len(result.Weights) != 2 {
		t.Fatalf("unexpected weight size, got=%d want=2", len(result.Weights))
	}
	if math.IsNaN(result.BIC) || math.IsInf(result.BIC, 0) {
		t.Fatalf("invalid BIC: %v", result.BIC)
	}

	normal := 0
	reversed := 0
	for i, label := range result.Labels {
		trueLabel := 0
		if i >= 120 {
			trueLabel = 1
		}
		if label == trueLabel {
			normal++
		}
		if label == 1-trueLabel {
			reversed++
		}
	}
	accuracy := float64(maxInt(normal, reversed)) / float64(len(vectors))
	if accuracy < 0.95 {
		t.Fatalf("unexpected clustering accuracy in auto-select mode, got=%0.4f want>=0.95", accuracy)
	}
}

func TestGMMClusterInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		vectors [][]float64
		k       int
		opts    *GMMOptions
		wantErr string
	}{
		{
			name:    "empty vectors",
			vectors: nil,
			k:       2,
			wantErr: "empty",
		},
		{
			name: "inconsistent dimensions",
			vectors: [][]float64{
				{1, 2},
				{3},
			},
			k:       2,
			wantErr: "same dimension",
		},
		{
			name: "invalid number value",
			vectors: [][]float64{
				{1, 2},
				{math.NaN(), 3},
			},
			k:       2,
			wantErr: "invalid value",
		},
		{
			name: "invalid cluster count",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k:       0,
			wantErr: "must not be 0",
		},
		{
			name: "cluster count exceeds sample count",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k:       3,
			wantErr: "must not exceed sample count",
		},
		{
			name: "negative options",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k: 2,
			opts: &GMMOptions{
				MaxIterations: -1,
			},
			wantErr: "must not be negative",
		},
		{
			name: "negative num initializations",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k: 2,
			opts: &GMMOptions{
				NumInitializations: -2,
			},
			wantErr: "num initializations must not be negative",
		},
		{
			name: "negative kmeans max iterations",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k: 2,
			opts: &GMMOptions{
				KMeansMaxIterations: -2,
			},
			wantErr: "kmeans max iterations must not be negative",
		},
		{
			name: "negative auto max clusters",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
			},
			k: -1,
			opts: &GMMOptions{
				AutoMaxClusters: -3,
			},
			wantErr: "auto max clusters must not be negative",
		},
		{
			name: "auto max clusters less than 2 in auto mode",
			vectors: [][]float64{
				{1, 2},
				{3, 4},
				{5, 6},
			},
			k: -1,
			opts: &GMMOptions{
				AutoMaxClusters: 1,
			},
			wantErr: "at least 2 in auto mode",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := GMMCluster(tc.vectors, tc.k, tc.opts)
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error message, got=%q want contains=%q", err.Error(), tc.wantErr)
			}
		})
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

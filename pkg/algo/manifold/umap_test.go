package manifold

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestUMAPFitTransform(t *testing.T) {
	data := generateUMAPBlobs(80, 6, 2, 7)

	model, err := NewUMAP(
		2,
		WithUMAPNNeighbors(10),
		WithUMAPInit(UMAPInitRandom),
		WithUMAPRandomSeed(42),
		WithUMAPNEpochs(80),
		WithUMAPNumWorkers(1),
	)
	if err != nil {
		t.Fatalf("new umap failed: %v", err)
	}

	embedding, err := model.FitTransform(data)
	if err != nil {
		t.Fatalf("fit transform failed: %v", err)
	}

	if len(embedding) != len(data) {
		t.Fatalf("embedding row mismatch: got=%d want=%d", len(embedding), len(data))
	}
	if len(embedding[0]) != 2 {
		t.Fatalf("embedding col mismatch: got=%d want=2", len(embedding[0]))
	}
	for i := range embedding {
		for j := range embedding[i] {
			value := embedding[i][j]
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatalf("embedding contains invalid value at [%d][%d]=%v", i, j, value)
			}
		}
	}
}

func TestUMAPDeterministicWithSeed(t *testing.T) {
	data := generateUMAPBlobs(60, 5, 3, 9)

	modelA, err := NewUMAP(
		2,
		WithUMAPNNeighbors(8),
		WithUMAPInit(UMAPInitRandom),
		WithUMAPRandomSeed(123),
		WithUMAPNEpochs(60),
		WithUMAPNumWorkers(1),
	)
	if err != nil {
		t.Fatalf("new umap A failed: %v", err)
	}
	embeddingA, err := modelA.FitTransform(data)
	if err != nil {
		t.Fatalf("fit transform A failed: %v", err)
	}

	modelB, err := NewUMAP(
		2,
		WithUMAPNNeighbors(8),
		WithUMAPInit(UMAPInitRandom),
		WithUMAPRandomSeed(123),
		WithUMAPNEpochs(60),
		WithUMAPNumWorkers(1),
	)
	if err != nil {
		t.Fatalf("new umap B failed: %v", err)
	}
	embeddingB, err := modelB.FitTransform(data)
	if err != nil {
		t.Fatalf("fit transform B failed: %v", err)
	}

	if len(embeddingA) != len(embeddingB) {
		t.Fatalf("embedding row mismatch: got=%d want=%d", len(embeddingA), len(embeddingB))
	}
	for i := range embeddingA {
		if len(embeddingA[i]) != len(embeddingB[i]) {
			t.Fatalf("embedding col mismatch at row=%d", i)
		}
		for j := range embeddingA[i] {
			diff := math.Abs(embeddingA[i][j] - embeddingB[i][j])
			if diff > 1e-6 {
				t.Fatalf("determinism mismatch at [%d][%d]: got=%v want=%v diff=%v", i, j, embeddingA[i][j], embeddingB[i][j], diff)
			}
		}
	}
}

func TestUMAPConfigValidation(t *testing.T) {
	data := generateUMAPBlobs(20, 4, 2, 17)

	staticCases := []struct {
		name    string
		newFunc func() error
		wantErr string
	}{
		{
			name: "invalid components",
			newFunc: func() error {
				_, err := NewUMAP(0)
				return err
			},
			wantErr: "n_components must be positive",
		},
		{
			name: "invalid metric",
			newFunc: func() error {
				_, err := NewUMAP(
					2,
					WithUMAPMetric("l2"),
				)
				return err
			},
			wantErr: "unsupported metric",
		},
		{
			name: "invalid set op mix ratio",
			newFunc: func() error {
				_, err := NewUMAP(
					2,
					WithUMAPSetOpMixRatio(1.2),
				)
				return err
			},
			wantErr: "set_op_mix_ratio must be within [0, 1]",
		},
	}

	for _, tc := range staticCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.newFunc()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: got=%q want contains=%q", err.Error(), tc.wantErr)
			}
		})
	}

	t.Run("invalid n_neighbors for current dataset", func(t *testing.T) {
		model, err := NewUMAP(
			2,
			WithUMAPNNeighbors(20),
		)
		if err != nil {
			t.Fatalf("new umap failed: %v", err)
		}
		_, err = model.FitTransform(data)
		if err == nil {
			t.Fatalf("expected error containing %q, got nil", "n_neighbors must be < n_samples")
		}
		if !strings.Contains(err.Error(), "n_neighbors must be < n_samples") {
			t.Fatalf("unexpected error: got=%q want contains=%q", err.Error(), "n_neighbors must be < n_samples")
		}
	})
}

func TestTSNEStaticConfigValidationOnNew(t *testing.T) {
	staticCases := []struct {
		name    string
		newFunc func() error
		wantErr string
	}{
		{
			name: "invalid components",
			newFunc: func() error {
				_, err := NewTSNE(0)
				return err
			},
			wantErr: "n_components must be positive",
		},
		{
			name: "invalid perplexity",
			newFunc: func() error {
				_, err := NewTSNE(2, WithPerplexity(0))
				return err
			},
			wantErr: "perplexity must be positive",
		},
	}

	for _, tc := range staticCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.newFunc()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error: got=%q want contains=%q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestTSNEDataDependentValidationOnFit(t *testing.T) {
	data := generateUMAPBlobs(20, 4, 2, 19)
	model, err := NewTSNE(2, WithPerplexity(50))
	if err != nil {
		t.Fatalf("new tsne failed: %v", err)
	}
	_, err = model.FitTransform(data)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", "perplexity")
	}
	if !strings.Contains(err.Error(), "perplexity") {
		t.Fatalf("unexpected error: got=%q want contains=%q", err.Error(), "perplexity")
	}
}

func TestUMAPEmbeddingReturnsCopy(t *testing.T) {
	data := generateUMAPBlobs(30, 5, 2, 23)

	model, err := NewUMAP(
		2,
		WithUMAPNNeighbors(6),
		WithUMAPInit(UMAPInitRandom),
		WithUMAPRandomSeed(4),
		WithUMAPNEpochs(40),
		WithUMAPNumWorkers(1),
	)
	if err != nil {
		t.Fatalf("new umap failed: %v", err)
	}
	if err := model.Fit(data); err != nil {
		t.Fatalf("fit failed: %v", err)
	}

	embeddingA := model.Embedding()
	embeddingA[0][0] += 12345
	embeddingB := model.Embedding()
	if embeddingA[0][0] == embeddingB[0][0] {
		t.Fatalf("embedding should be cloned copy, but mutation leaked")
	}
}

func generateUMAPBlobs(nSamples int, nFeatures int, nClusters int, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	result := make([][]float64, nSamples)
	clusterSize := max(nSamples/nClusters, 1)
	for i := 0; i < nSamples; i++ {
		clusterIdx := i / clusterSize
		if clusterIdx >= nClusters {
			clusterIdx = nClusters - 1
		}
		center := float64(clusterIdx) * 8.0
		row := make([]float64, nFeatures)
		for j := range row {
			row[j] = center + rng.NormFloat64()*0.5
		}
		result[i] = row
	}
	return result
}

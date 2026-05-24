package test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
)

func TestAutoSelectGaussianMixture_ByBIC(t *testing.T) {
	data := generateGaussianBlobs(
		[]blobConfig{
			{CX: -6, CY: -6, Std: 0.45, N: 90},
			{CX: 0, CY: 0, Std: 0.50, N: 90},
			{CX: 6, CY: 6, Std: 0.40, N: 90},
		},
		42,
	)

	model, evaluation, selection, err := mixture.AutoSelectGaussianMixture(
		data,
		2,
		5,
		mixture.AutoSelectionCriterionBIC,
		mixture.WithNInit(3),
		mixture.WithMaxIterations(120),
		mixture.WithTolerance(1e-4),
		mixture.WithRegularization(1e-6),
		mixture.WithInitParams(mixture.InitParamsKMeans),
		mixture.WithRandomSeed(42),
	)
	if err != nil {
		t.Fatalf("auto select gaussian mixture failed: %v", err)
	}
	if model == nil {
		t.Fatalf("expected selected model, got nil")
	}
	if selection.SelectedComponents != 3 {
		t.Fatalf("selected components mismatch: got=%d want=%d", selection.SelectedComponents, 3)
	}
	if len(selection.Candidates) != 4 {
		t.Fatalf("candidate count mismatch: got=%d want=%d", len(selection.Candidates), 4)
	}
	if len(evaluation.Labels) != len(data) {
		t.Fatalf("labels size mismatch: got=%d want=%d", len(evaluation.Labels), len(data))
	}
}

func TestAutoSelectGaussianMixture_InvalidRange(t *testing.T) {
	data := generateGaussianBlobs(
		[]blobConfig{
			{CX: 0, CY: 0, Std: 0.5, N: 20},
			{CX: 3, CY: 3, Std: 0.5, N: 20},
		},
		7,
	)

	_, _, _, err := mixture.AutoSelectGaussianMixture(
		data,
		5,
		2,
		mixture.AutoSelectionCriterionBIC,
	)
	if err == nil {
		t.Fatalf("expected invalid range error, got nil")
	}
	if !strings.Contains(err.Error(), "max_components must be >= min_components") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutoSelectGaussianMixture_Default(t *testing.T) {
	data := generateGaussianBlobs(
		[]blobConfig{
			{CX: -4, CY: -4, Std: 0.5, N: 60},
			{CX: 4, CY: 4, Std: 0.5, N: 60},
		},
		11,
	)

	model, evaluation, selection, err := mixture.AutoSelectGaussianMixtureDefault(
		data,
		mixture.WithNInit(2),
		mixture.WithMaxIterations(100),
		mixture.WithRandomSeed(11),
	)
	if err != nil {
		t.Fatalf("default auto select failed: %v", err)
	}
	if model == nil {
		t.Fatalf("expected selected model, got nil")
	}
	if selection.Criterion != mixture.AutoSelectionCriterionBIC {
		t.Fatalf("criterion mismatch: got=%q", selection.Criterion)
	}
	if selection.MinComponents != 1 {
		t.Fatalf("min components mismatch: got=%d want=1", selection.MinComponents)
	}
	if selection.MaxComponents <= 0 || selection.MaxComponents > len(data) {
		t.Fatalf("max components out of range: %d", selection.MaxComponents)
	}
	if len(evaluation.Labels) != len(data) {
		t.Fatalf("labels size mismatch: got=%d want=%d", len(evaluation.Labels), len(data))
	}
}

type blobConfig struct {
	CX  float64
	CY  float64
	Std float64
	N   int
}

func generateGaussianBlobs(blobs []blobConfig, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	total := 0
	for _, blob := range blobs {
		total += blob.N
	}
	data := make([][]float64, 0, total)
	for _, blob := range blobs {
		for i := 0; i < blob.N; i++ {
			data = append(data, []float64{
				blob.CX + rng.NormFloat64()*blob.Std,
				blob.CY + rng.NormFloat64()*blob.Std,
			})
		}
	}
	return data
}

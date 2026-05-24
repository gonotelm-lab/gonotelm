package normalize

import (
	"fmt"
	"math"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

const defaultEpsilon = 1e-12

// L2 scales each row vector to unit L2 norm.
//
// Rows with norm <= epsilon are kept unchanged to avoid divide-by-zero.
func L2(data [][]float64) ([][]float64, error) {
	return L2WithEpsilon(data, defaultEpsilon)
}

// L2WithEpsilon scales each row vector to unit L2 norm with custom epsilon.
func L2WithEpsilon(data [][]float64, epsilon float64) ([][]float64, error) {
	_, _, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return nil, err
	}
	if epsilon < 0 || math.IsNaN(epsilon) || math.IsInf(epsilon, 0) {
		return nil, fmt.Errorf("epsilon must be finite and >= 0, got %v", epsilon)
	}

	normalized := numutil.Clone2DFloat64(data)
	for rowIdx, row := range normalized {
		norm := floats.Norm(row, 2)
		if norm <= epsilon {
			continue
		}
		floats.Scale(1.0/norm, row)
		normalized[rowIdx] = row
	}
	return normalized, nil
}

// ZScore performs per-column standardization and returns normalized vectors.
func ZScore(data [][]float64) ([][]float64, error) {
	normalized, _, _, err := ZScoreWithStats(data)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

// ZScoreWithStats performs per-column standardization and returns statistics.
//
// For columns with near-zero variance, std is treated as 1 and output becomes 0-centered.
func ZScoreWithStats(data [][]float64) (normalized [][]float64, means []float64, stds []float64, err error) {
	_, nFeatures, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return nil, nil, nil, err
	}

	means = make([]float64, nFeatures)
	stds = make([]float64, nFeatures)
	for featureIdx := 0; featureIdx < nFeatures; featureIdx++ {
		column := numutil.ColumnFrom2DFloat64(data, featureIdx)
		means[featureIdx] = stat.Mean(column, nil)
		std := stat.PopStdDev(column, nil)
		if std <= defaultEpsilon {
			std = 1.0
		}
		stds[featureIdx] = std
	}

	normalized, err = ApplyZScore(data, means, stds)
	if err != nil {
		return nil, nil, nil, err
	}
	return normalized, means, stds, nil
}

// ApplyZScore applies precomputed z-score parameters to data.
func ApplyZScore(data [][]float64, means []float64, stds []float64) ([][]float64, error) {
	_, nFeatures, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return nil, err
	}
	if len(means) != nFeatures || len(stds) != nFeatures {
		return nil, fmt.Errorf(
			"zscore parameter length mismatch: features=%d means=%d stds=%d",
			nFeatures,
			len(means),
			len(stds),
		)
	}
	for idx, std := range stds {
		if std <= 0 || math.IsNaN(std) || math.IsInf(std, 0) {
			return nil, fmt.Errorf("stds[%d] must be finite and > 0, got %v", idx, std)
		}
	}

	normalized := numutil.Clone2DFloat64(data)
	for sampleIdx := range normalized {
		for featureIdx := 0; featureIdx < nFeatures; featureIdx++ {
			normalized[sampleIdx][featureIdx] =
				(normalized[sampleIdx][featureIdx] - means[featureIdx]) / stds[featureIdx]
		}
	}
	return normalized, nil
}

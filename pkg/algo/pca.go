package algo

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// PCAReduceVector uses PCA to reduce a 1D vector to targetDim length.
//
// Because PCA expects a 2D matrix (rows=samples, cols=features), this function
// reshapes the input vector into a matrix with `targetDim` rows and
// `len(input)/targetDim` columns, then projects each row onto the first
// principal component, resulting in a targetDim-sized vector.
func PCAReduceVector(input []float64, targetDim int) ([]float64, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("input vector is empty")
	}
	if targetDim <= 0 {
		return nil, fmt.Errorf("target dimension must be greater than 0")
	}
	if targetDim < 2 {
		return nil, fmt.Errorf("target dimension must be at least 2 for PCA")
	}
	if targetDim >= len(input) {
		return nil, fmt.Errorf("target dimension must be less than input length")
	}
	if len(input)%targetDim != 0 {
		return nil, fmt.Errorf("input length must be divisible by target dimension")
	}

	featureDim := len(input) / targetDim
	if featureDim < 2 {
		return nil, fmt.Errorf("feature dimension must be at least 2 for PCA")
	}

	data := append([]float64(nil), input...)
	samples := mat.NewDense(targetDim, featureDim, data)

	return reduceByFirstPrincipalComponent(samples)
}

func reduceByFirstPrincipalComponent(samples *mat.Dense) ([]float64, error) {
	rows, cols := samples.Dims()
	if rows < 2 || cols < 2 {
		return nil, fmt.Errorf("matrix must have at least 2 rows and 2 columns")
	}

	var pc stat.PC
	if ok := pc.PrincipalComponents(samples, nil); !ok {
		return nil, fmt.Errorf("principal components analysis failed")
	}

	means := make([]float64, cols)
	for c := 0; c < cols; c++ {
		col := mat.Col(nil, c, samples)
		means[c] = stat.Mean(col, nil)
	}

	centeredData := make([]float64, rows*cols)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			centeredData[r*cols+c] = samples.At(r, c) - means[c]
		}
	}
	centered := mat.NewDense(rows, cols, centeredData)

	var components mat.Dense
	pc.VectorsTo(&components)
	firstPC := components.Slice(0, cols, 0, 1)

	var reduced mat.Dense
	reduced.Mul(centered, firstPC)

	out := mat.Col(nil, 0, &reduced)
	for idx, v := range out {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid reduced value at index %d: %v", idx, v)
		}
	}

	return out, nil
}


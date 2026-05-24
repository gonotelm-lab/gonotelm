package numutil

import (
	"fmt"
	"math"
	"slices"

	"gonum.org/v1/gonum/mat"
)

// Validate2DFloat64 validates shape and numeric sanity for 2D float data.
//
// minSamples <= 0 means no additional lower bound constraint.
func Validate2DFloat64(data [][]float64, minSamples int) (nSamples int, nFeatures int, err error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("input data is empty")
	}

	nFeatures = len(data[0])
	if nFeatures == 0 {
		return 0, 0, fmt.Errorf("input data has zero features")
	}

	for i, row := range data {
		if len(row) != nFeatures {
			return 0, 0, fmt.Errorf("row %d has %d features, expected %d", i, len(row), nFeatures)
		}
		for j, value := range row {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return 0, 0, fmt.Errorf("input contains invalid value at row %d, col %d", i, j)
			}
		}
	}

	nSamples = len(data)
	if minSamples > 0 && nSamples < minSamples {
		return 0, 0, fmt.Errorf("input requires at least %d samples, got %d", minSamples, nSamples)
	}

	return nSamples, nFeatures, nil
}

// ValidateRowsWithFeatureCount validates data rows against expected feature count.
func ValidateRowsWithFeatureCount(data [][]float64, expectedFeatures int) error {
	if len(data) == 0 {
		return fmt.Errorf("input data is empty")
	}
	if expectedFeatures <= 0 {
		return fmt.Errorf("expected feature count must be positive, got %d", expectedFeatures)
	}

	for i, row := range data {
		if len(row) != expectedFeatures {
			return fmt.Errorf("row %d has %d features, expected %d", i, len(row), expectedFeatures)
		}
		for j, value := range row {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("input contains invalid value at row %d, col %d", i, j)
			}
		}
	}

	return nil
}

// ColumnMean computes per-feature means.
func ColumnMean(data [][]float64, nFeatures int) []float64 {
	mean := make([]float64, nFeatures)
	for _, row := range data {
		for j, value := range row {
			mean[j] += value
		}
	}
	nSamplesScale := float64(len(data))
	for j := range mean {
		mean[j] /= nSamplesScale
	}
	return mean
}

// CenterToDense subtracts feature mean and returns a dense matrix.
func CenterToDense(data [][]float64, mean []float64) *mat.Dense {
	nSamples := len(data)
	nFeatures := len(mean)
	centered := mat.NewDense(nSamples, nFeatures, nil)
	for i, row := range data {
		for j, value := range row {
			centered.Set(i, j, value-mean[j])
		}
	}
	return centered
}

// DenseToSlice converts any matrix to [][]float64 copy.
func DenseToSlice(matrix mat.Matrix) [][]float64 {
	switch typed := matrix.(type) {
	case *mat.Dense:
		return denseToSliceFast(typed)
	case *mat.SymDense:
		return denseToSliceFast(mat.DenseCopyOf(typed))
	default:
		nRows, nCols := matrix.Dims()
		result := make([][]float64, nRows)
		for rowIdx := 0; rowIdx < nRows; rowIdx++ {
			result[rowIdx] = make([]float64, nCols)
			for colIdx := 0; colIdx < nCols; colIdx++ {
				result[rowIdx][colIdx] = matrix.At(rowIdx, colIdx)
			}
		}
		return result
	}
}

func denseToSliceFast(matrix *mat.Dense) [][]float64 {
	nRows, _ := matrix.Dims()
	result := make([][]float64, nRows)
	for rowIdx := 0; rowIdx < nRows; rowIdx++ {
		result[rowIdx] = slices.Clone(matrix.RawRowView(rowIdx))
	}
	return result
}

// Flatten2DFloat64 flattens a 2D matrix in row-major order.
func Flatten2DFloat64(matrix [][]float64) []float64 {
	if len(matrix) == 0 {
		return nil
	}
	nRows := len(matrix)
	nCols := len(matrix[0])
	flattened := make([]float64, 0, nRows*nCols)
	for _, row := range matrix {
		flattened = append(flattened, row...)
	}
	return flattened
}

// ReshapeRowMajorFloat64 reshapes row-major parameters into [][]float64.
func ReshapeRowMajorFloat64(params []float64, nRows int, nCols int) [][]float64 {
	result := make([][]float64, nRows)
	for rowIdx := 0; rowIdx < nRows; rowIdx++ {
		rowStart := rowIdx * nCols
		result[rowIdx] = make([]float64, nCols)
		copy(result[rowIdx], params[rowStart:rowStart+nCols])
	}
	return result
}

// ColumnFrom2DFloat64 extracts one column from a 2D matrix.
func ColumnFrom2DFloat64(matrix [][]float64, columnIndex int) []float64 {
	result := make([]float64, len(matrix))
	for rowIdx := range matrix {
		result[rowIdx] = matrix[rowIdx][columnIndex]
	}
	return result
}

// Float64ToFloat32Matrix converts [][]float64 to [][]float32 with contiguous rows.
func Float64ToFloat32Matrix(data [][]float64) [][]float32 {
	if len(data) == 0 {
		return nil
	}
	nRows := len(data)
	nCols := len(data[0])

	flat := make([]float32, nRows*nCols)
	result := make([][]float32, nRows)
	for rowIdx := 0; rowIdx < nRows; rowIdx++ {
		rowStart := rowIdx * nCols
		rowView := flat[rowStart : rowStart+nCols]
		for colIdx := 0; colIdx < nCols; colIdx++ {
			rowView[colIdx] = float32(data[rowIdx][colIdx])
		}
		result[rowIdx] = rowView
	}
	return result
}

// Float32ToFloat64Matrix converts [][]float32 to [][]float64 with contiguous rows.
func Float32ToFloat64Matrix(data [][]float32) [][]float64 {
	if len(data) == 0 {
		return nil
	}
	nRows := len(data)
	nCols := len(data[0])

	flat := make([]float64, nRows*nCols)
	result := make([][]float64, nRows)
	for rowIdx := 0; rowIdx < nRows; rowIdx++ {
		rowStart := rowIdx * nCols
		rowView := flat[rowStart : rowStart+nCols]
		for colIdx := 0; colIdx < nCols; colIdx++ {
			rowView[colIdx] = float64(data[rowIdx][colIdx])
		}
		result[rowIdx] = rowView
	}
	return result
}

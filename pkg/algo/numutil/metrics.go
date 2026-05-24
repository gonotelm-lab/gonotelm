package numutil

import (
	"math"
	"slices"
	"sort"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

// Clone2DFloat64 deep-copies a [][]float64 matrix.
func Clone2DFloat64(matrix [][]float64) [][]float64 {
	result := make([][]float64, len(matrix))
	for rowIdx := range matrix {
		result[rowIdx] = slices.Clone(matrix[rowIdx])
	}
	return result
}

// TruncateFeatureDim truncates each sample to at most featureLimit dimensions.
// featureLimit <= 0 returns a full deep copy.
func TruncateFeatureDim(vectors [][]float64, featureLimit int) [][]float64 {
	if featureLimit <= 0 {
		return Clone2DFloat64(vectors)
	}

	result := make([][]float64, len(vectors))
	for sampleIdx, sample := range vectors {
		dim := min(featureLimit, len(sample))
		result[sampleIdx] = make([]float64, dim)
		copy(result[sampleIdx], sample[:dim])
	}
	return result
}

// CondensedPairwiseDistances computes upper-triangular pairwise Euclidean distances.
func CondensedPairwiseDistances(vectors [][]float64) []float64 {
	nSamples := len(vectors)
	if nSamples < 2 {
		return nil
	}

	distances := make([]float64, 0, nSamples*(nSamples-1)/2)
	for i := 0; i < nSamples; i++ {
		for j := i + 1; j < nSamples; j++ {
			distances = append(distances, floats.Distance(vectors[i], vectors[j], 2))
		}
	}
	return distances
}

// NormalizeByMean scales values by their arithmetic mean.
func NormalizeByMean(values []float64) []float64 {
	result := make([]float64, len(values))
	if len(values) == 0 {
		return result
	}
	mean := floats.Sum(values) / float64(len(values))
	if mean == 0 {
		copy(result, values)
		return result
	}
	copy(result, values)
	floats.Scale(1.0/mean, result)
	return result
}

// PearsonCorrelation returns Pearson's r. Invalid inputs return 0.
func PearsonCorrelation(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	correlation := stat.Correlation(a, b, nil)
	if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
		return 0
	}
	return correlation
}

// Trustworthiness computes sklearn-compatible trustworthiness score in [0,1].
func Trustworthiness(high, low [][]float64, nNeighbors int) float64 {
	nSamples := len(high)
	if nSamples == 0 || nNeighbors <= 0 || nNeighbors >= nSamples {
		return 0
	}

	highDistances := pairwiseDistances(high)
	lowDistances := pairwiseDistances(low)

	highRank := make([][]int, nSamples)
	for i := 0; i < nSamples; i++ {
		order := argsortByDistance(highDistances[i], i)
		rank := make([]int, nSamples)
		for idx, sampleIdx := range order {
			rank[sampleIdx] = idx + 1
		}
		highRank[i] = rank
	}

	penalty := 0.0
	for i := 0; i < nSamples; i++ {
		lowOrder := argsortByDistance(lowDistances[i], i)
		for k := 0; k < nNeighbors; k++ {
			neighbor := lowOrder[k]
			r := highRank[i][neighbor]
			if r > nNeighbors {
				penalty += float64(r - nNeighbors)
			}
		}
	}

	n := float64(nSamples)
	k := float64(nNeighbors)
	scale := 2.0 / (n * k * (2.0*n - 3.0*k - 1.0))
	return 1.0 - penalty*scale
}

// AdjustedRandIndex computes clustering similarity score in [-1,1].
func AdjustedRandIndex(labelsA, labelsB []int) float64 {
	if len(labelsA) != len(labelsB) || len(labelsA) == 0 {
		return math.NaN()
	}
	if len(labelsA) == 1 {
		return 1
	}

	contingency := make(map[[2]int]int, len(labelsA))
	rowTotals := make(map[int]int, len(labelsA))
	colTotals := make(map[int]int, len(labelsA))

	for idx := 0; idx < len(labelsA); idx++ {
		a := labelsA[idx]
		b := labelsB[idx]
		rowTotals[a]++
		colTotals[b]++
		contingency[[2]int{a, b}]++
	}

	sumCombContingency := 0.0
	for _, value := range contingency {
		sumCombContingency += comb2(value)
	}

	sumCombRows := 0.0
	for _, value := range rowTotals {
		sumCombRows += comb2(value)
	}

	sumCombCols := 0.0
	for _, value := range colTotals {
		sumCombCols += comb2(value)
	}

	totalComb := comb2(len(labelsA))
	expected := (sumCombRows * sumCombCols) / totalComb
	maxIndex := 0.5 * (sumCombRows + sumCombCols)
	denominator := maxIndex - expected
	if denominator == 0 {
		return 1
	}
	return (sumCombContingency - expected) / denominator
}

func pairwiseDistances(data [][]float64) [][]float64 {
	nSamples := len(data)
	distances := make([][]float64, nSamples)
	for i := 0; i < nSamples; i++ {
		distances[i] = make([]float64, nSamples)
	}
	for i := 0; i < nSamples; i++ {
		for j := i + 1; j < nSamples; j++ {
			value := floats.Distance(data[i], data[j], 2)
			distances[i][j] = value
			distances[j][i] = value
		}
		distances[i][i] = math.Inf(1)
	}
	return distances
}

func argsortByDistance(row []float64, self int) []int {
	type pair struct {
		index int
		value float64
	}
	pairs := make([]pair, 0, len(row)-1)
	for idx, value := range row {
		if idx == self {
			continue
		}
		pairs = append(pairs, pair{index: idx, value: value})
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].value == pairs[j].value {
			return pairs[i].index < pairs[j].index
		}
		return pairs[i].value < pairs[j].value
	})

	result := make([]int, len(pairs))
	for i := range pairs {
		result[i] = pairs[i].index
	}
	return result
}

func comb2(value int) float64 {
	if value < 2 {
		return 0
	}
	return float64(value*(value-1)) / 2
}

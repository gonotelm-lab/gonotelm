package decomposition

import (
	"fmt"
	"math"
	"slices"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// Option configures PCA behavior.
type Option func(*PCA)

// WithWhiten enables or disables whitening in Transform.
func WithWhiten(enabled bool) Option {
	return func(p *PCA) {
		p.whiten = enabled
	}
}

// PCA is a dense-matrix principal component analysis implementation.
//
// The implementation follows the same core formulation as sklearn:
// - center each feature
// - perform full SVD on centered data
// - compute explained variance from singular values
type PCA struct {
	requestedComponents int
	whiten              bool

	fitted      bool
	nSamples    int
	nFeatures   int
	nComponents int

	mean                   []float64
	components             *mat.Dense // (n_components, n_features)
	explainedVariance      []float64
	explainedVarianceRatio []float64
	singularValues         []float64
	noiseVariance          float64
}

// NewPCA creates a PCA model.
//
// nComponents <= 0 means keeping all available components (min(n_samples, n_features)).
func NewPCA(nComponents int, opts ...Option) *PCA {
	p := &PCA{
		requestedComponents: nComponents,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Fit estimates PCA components from input samples.
func (p *PCA) Fit(data [][]float64) error {
	nSamples, nFeatures, err := numutil.Validate2DFloat64(data, 2)
	if err != nil {
		return err
	}

	minDims := min(nSamples, nFeatures)
	nComponents, err := resolveComponentCount(p.requestedComponents, minDims)
	if err != nil {
		return err
	}

	mean := numutil.ColumnMean(data, nFeatures)
	centered := numutil.CenterToDense(data, mean)

	var svd mat.SVD
	if ok := svd.Factorize(centered, mat.SVDThin); !ok {
		return fmt.Errorf("pca factorization failed")
	}

	singularValuesAll := svd.Values(nil)
	if len(singularValuesAll) == 0 {
		return fmt.Errorf("pca factorization returned empty singular values")
	}

	var v mat.Dense
	svd.VTo(&v) // (n_features, min_dims)
	componentsAll := mat.DenseCopyOf(v.T())
	svdFlipRows(componentsAll)

	explainedVarianceAll := make([]float64, len(singularValuesAll))
	var totalVariance float64
	for i, singularValue := range singularValuesAll {
		value := (singularValue * singularValue) / float64(nSamples-1)
		explainedVarianceAll[i] = value
		totalVariance += value
	}

	explainedVarianceRatioAll := make([]float64, len(explainedVarianceAll))
	if totalVariance > 0 {
		for i, variance := range explainedVarianceAll {
			explainedVarianceRatioAll[i] = variance / totalVariance
		}
	}

	components := mat.NewDense(nComponents, nFeatures, nil)
	components.Copy(componentsAll.Slice(0, nComponents, 0, nFeatures))

	noiseVariance := 0.0
	if nComponents < len(explainedVarianceAll) {
		tail := explainedVarianceAll[nComponents:]
		noiseVariance = floats.Sum(tail) / float64(len(tail))
	}

	p.fitted = true
	p.nSamples = nSamples
	p.nFeatures = nFeatures
	p.nComponents = nComponents
	p.mean = mean
	p.components = components
	p.explainedVariance = slices.Clone(explainedVarianceAll[:nComponents])
	p.explainedVarianceRatio = slices.Clone(explainedVarianceRatioAll[:nComponents])
	p.singularValues = slices.Clone(singularValuesAll[:nComponents])
	p.noiseVariance = noiseVariance
	return nil
}

// Transform projects samples into principal component space.
func (p *PCA) Transform(data [][]float64) ([][]float64, error) {
	if err := p.ensureTransformable(data); err != nil {
		return nil, err
	}

	centered := numutil.CenterToDense(data, p.mean)
	var projected mat.Dense
	projected.Mul(centered, p.components.T())

	if p.whiten {
		whitenInPlace(&projected, p.explainedVariance)
	}

	return numutil.DenseToSlice(&projected), nil
}

// FitTransform is equivalent to Fit followed by Transform.
func (p *PCA) FitTransform(data [][]float64) ([][]float64, error) {
	if err := p.Fit(data); err != nil {
		return nil, err
	}
	return p.Transform(data)
}

// Mean returns per-feature empirical means from the training set.
func (p *PCA) Mean() []float64 {
	return slices.Clone(p.mean)
}

// Components returns principal axes in feature space.
func (p *PCA) Components() [][]float64 {
	if p.components == nil {
		return nil
	}
	return numutil.DenseToSlice(p.components)
}

// ExplainedVariance returns variance explained by each selected component.
func (p *PCA) ExplainedVariance() []float64 {
	return slices.Clone(p.explainedVariance)
}

// ExplainedVarianceRatio returns explained variance percentages.
func (p *PCA) ExplainedVarianceRatio() []float64 {
	return slices.Clone(p.explainedVarianceRatio)
}

// SingularValues returns selected singular values.
func (p *PCA) SingularValues() []float64 {
	return slices.Clone(p.singularValues)
}

// NoiseVariance returns the Probabilistic PCA residual variance estimate.
func (p *PCA) NoiseVariance() float64 {
	return p.noiseVariance
}

// NComponents returns fitted component count.
func (p *PCA) NComponents() int {
	return p.nComponents
}

func (p *PCA) ensureTransformable(data [][]float64) error {
	if !p.fitted {
		return fmt.Errorf("pca model is not fitted")
	}
	return numutil.ValidateRowsWithFeatureCount(data, p.nFeatures)
}

func resolveComponentCount(requested, maxComponents int) (int, error) {
	if requested <= 0 {
		return maxComponents, nil
	}
	if requested > maxComponents {
		return 0, fmt.Errorf("n_components=%d exceeds max=%d", requested, maxComponents)
	}
	return requested, nil
}

// svdFlipRows mirrors sklearn's svd_flip(u_based_decision=False) behavior.
func svdFlipRows(components *mat.Dense) {
	nRows, nCols := components.Dims()
	for rowIdx := range nRows {
		row := components.RawRowView(rowIdx)
		maxAbsCol := 0
		maxAbsValue := math.Abs(row[0])
		for colIdx := range nCols {
			currentAbs := math.Abs(row[colIdx])
			if currentAbs > maxAbsValue {
				maxAbsValue = currentAbs
				maxAbsCol = colIdx
			}
		}
		if row[maxAbsCol] < 0 {
			for colIdx := range row {
				row[colIdx] = -row[colIdx]
			}
		}
	}
}

func whitenInPlace(projected *mat.Dense, explainedVariance []float64) {
	const machineEpsilon = 2.220446049250313e-16 // IEEE-754 float64 epsilon.

	nSamples, nComponents := projected.Dims()
	for componentIdx := range nComponents {
		scale := math.Sqrt(explainedVariance[componentIdx])
		if scale < machineEpsilon {
			scale = machineEpsilon
		}
		for sampleIdx := range nSamples {
			projected.Set(sampleIdx, componentIdx, projected.At(sampleIdx, componentIdx)/scale)
		}
	}
}

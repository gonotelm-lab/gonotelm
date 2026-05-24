package mixture

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distmv"
)

const (
	defaultTolerance      = 1e-3
	defaultRegularization = 1e-6
	defaultMaxIterations  = 100
	defaultNInit          = 1
	defaultSeed           = uint64(7)
	defaultAutoMinK       = 1
	defaultAutoMaxK       = 10

	minComponentMass = 1e-12
	initSmoothing    = 1e-3
)

// InitParams controls Gaussian mixture initialization behavior.
type InitParams string

const (
	// InitParamsKMeans uses kmeans++ centers + hard assignments.
	InitParamsKMeans InitParams = "kmeans"
	// InitParamsRandom samples random soft responsibilities.
	InitParamsRandom InitParams = "random"
)

// Option configures GaussianMixture behavior.
type Option func(*GaussianMixture)

// WithTolerance sets the EM convergence threshold.
func WithTolerance(tolerance float64) Option {
	return func(gm *GaussianMixture) {
		gm.tolerance = tolerance
	}
}

// WithRegularization sets the diagonal regularization added to each covariance matrix.
func WithRegularization(regularization float64) Option {
	return func(gm *GaussianMixture) {
		gm.regularization = regularization
	}
}

// WithMaxIterations sets the EM loop upper bound.
func WithMaxIterations(maxIterations int) Option {
	return func(gm *GaussianMixture) {
		gm.maxIterations = maxIterations
	}
}

// WithNInit sets the number of random initializations.
func WithNInit(nInit int) Option {
	return func(gm *GaussianMixture) {
		gm.nInit = nInit
	}
}

// WithInitParams sets the initialization strategy.
func WithInitParams(initParams InitParams) Option {
	return func(gm *GaussianMixture) {
		gm.initParams = initParams
	}
}

// WithRandomSeed sets the deterministic random seed.
func WithRandomSeed(seed uint64) Option {
	return func(gm *GaussianMixture) {
		gm.seed = seed
	}
}

// GaussianMixture is a full-covariance Gaussian Mixture Model using EM.
//
// The implementation follows sklearn.mixture.GaussianMixture core steps:
// - E step: evaluate weighted component log probabilities
// - M step: update weights / means / full covariances
// - stop when average lower-bound improvement <= tolerance
type GaussianMixture struct {
	nComponents int

	tolerance      float64
	regularization float64
	maxIterations  int
	nInit          int
	initParams     InitParams
	seed           uint64

	fitted      bool
	nSamples    int
	nFeatures   int
	weights     []float64
	means       [][]float64
	covariances []*mat.SymDense
	normals     []*distmv.Normal

	converged   bool
	iterations  int
	lowerBound  float64
	lowerBounds []float64
}

type fitState struct {
	weights     []float64
	means       [][]float64
	covariances []*mat.SymDense

	converged   bool
	iterations  int
	lowerBound  float64
	lowerBounds []float64
}

// Evaluation summarizes inference statistics for a fitted GaussianMixture.
//
// LogLikelihood is the total log-likelihood on the provided dataset.
// AverageLogLikelihood is the per-sample mean log-likelihood (same value as Score()).
type Evaluation struct {
	// Labels stores the predicted cluster index for each input sample, aligned by row index.
	//
	// Example:
	// - input vectors: [x0, x1, x2, x3]
	// - Labels:        [1, 0, 1, 2]
	//
	// Meaning:
	// - x0 -> cluster 1
	// - x1 -> cluster 0
	// - x2 -> cluster 1
	// - x3 -> cluster 2
	Labels               []int
	// Weights stores the mixture weight for each cluster index.
	//
	// Example:
	// - Weights: [0.25, 0.50, 0.25]
	// - Meaning: cluster 0/1/2 occupies about 25% / 50% / 25% probability mass.
	Weights              []float64
	// Means stores the centroid vector for each cluster index.
	//
	// Example:
	// - Means[0] is cluster 0 center
	// - Means[1] is cluster 1 center
	// - Means[2] is cluster 2 center
	//
	// `Means[i]` aligns with `Weights[i]` and cluster id `i` in Labels.
	Means                [][]float64
	LogLikelihood        float64
	AverageLogLikelihood float64
	BIC                  float64
	AIC                  float64
	Iterations           int
	Converged            bool
}

// AutoSelectionCriterion controls the metric used for automatic component-count selection.
type AutoSelectionCriterion string

const (
	// AutoSelectionCriterionBIC chooses the model with the smallest BIC.
	AutoSelectionCriterionBIC AutoSelectionCriterion = "bic"
	// AutoSelectionCriterionAIC chooses the model with the smallest AIC.
	AutoSelectionCriterionAIC AutoSelectionCriterion = "aic"
)

// ComponentCandidateScore stores one candidate K score during auto-selection.
type ComponentCandidateScore struct {
	Components    int
	BIC           float64
	AIC           float64
	LogLikelihood float64
	Iterations    int
	Converged     bool
}

// AutoSelectionResult summarizes automatic component-count search.
type AutoSelectionResult struct {
	Criterion          AutoSelectionCriterion
	MinComponents      int
	MaxComponents      int
	SelectedComponents int
	SelectedScore      float64
	Candidates         []ComponentCandidateScore
}

// NewGaussianMixture creates a GaussianMixture.
func NewGaussianMixture(nComponents int, opts ...Option) (*GaussianMixture, error) {
	gm := &GaussianMixture{
		nComponents:    nComponents,
		tolerance:      defaultTolerance,
		regularization: defaultRegularization,
		maxIterations:  defaultMaxIterations,
		nInit:          defaultNInit,
		initParams:     InitParamsKMeans,
		seed:           defaultSeed,
	}
	for _, opt := range opts {
		opt(gm)
	}
	if err := gm.validateStaticHyperParams(); err != nil {
		return nil, fmt.Errorf("invalid gaussian mixture config: %w", err)
	}
	return gm, nil
}

// AutoSelectGaussianMixture fits multiple component counts and returns the best model.
//
// The search range is [minComponents, maxComponents], inclusive.
// Criterion "bic" or "aic" picks the smallest score.
func AutoSelectGaussianMixture(
	data [][]float64,
	minComponents int,
	maxComponents int,
	criterion AutoSelectionCriterion,
	opts ...Option,
) (*GaussianMixture, Evaluation, AutoSelectionResult, error) {
	nSamples, _, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return nil, Evaluation{}, AutoSelectionResult{}, err
	}
	if err := validateAutoSelectionRange(minComponents, maxComponents, nSamples); err != nil {
		return nil, Evaluation{}, AutoSelectionResult{}, err
	}
	if err := validateAutoSelectionCriterion(criterion); err != nil {
		return nil, Evaluation{}, AutoSelectionResult{}, err
	}

	result := AutoSelectionResult{
		Criterion:     criterion,
		MinComponents: minComponents,
		MaxComponents: maxComponents,
		Candidates:    make([]ComponentCandidateScore, 0, maxComponents-minComponents+1),
	}

	var bestModel *GaussianMixture
	var bestEvaluation Evaluation
	bestScore := math.Inf(1)

	for nComponents := minComponents; nComponents <= maxComponents; nComponents++ {
		model, err := NewGaussianMixture(nComponents, opts...)
		if err != nil {
			return nil, Evaluation{}, AutoSelectionResult{}, err
		}
		evaluation, err := model.FitEvaluate(data)
		if err != nil {
			return nil, Evaluation{}, AutoSelectionResult{}, fmt.Errorf(
				"fit failed for n_components=%d: %w",
				nComponents,
				err,
			)
		}

		result.Candidates = append(result.Candidates, ComponentCandidateScore{
			Components:    nComponents,
			BIC:           evaluation.BIC,
			AIC:           evaluation.AIC,
			LogLikelihood: evaluation.LogLikelihood,
			Iterations:    evaluation.Iterations,
			Converged:     evaluation.Converged,
		})

		score := selectScoreByCriterion(evaluation, criterion)
		if score < bestScore {
			bestScore = score
			bestModel = model
			bestEvaluation = cloneEvaluation(evaluation)
		}
	}

	if bestModel == nil {
		return nil, Evaluation{}, AutoSelectionResult{}, fmt.Errorf("no valid model found in selection range")
	}
	result.SelectedComponents = bestModel.nComponents
	result.SelectedScore = bestScore
	return bestModel, bestEvaluation, result, nil
}

// AutoSelectGaussianMixtureDefault auto-selects K with built-in defaults.
//
// Defaults:
// - criterion: BIC
// - search range: [1, min(10, n_samples)]
func AutoSelectGaussianMixtureDefault(
	data [][]float64,
	opts ...Option,
) (*GaussianMixture, Evaluation, AutoSelectionResult, error) {
	nSamples, _, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return nil, Evaluation{}, AutoSelectionResult{}, err
	}
	maxComponents := min(defaultAutoMaxK, nSamples)
	return AutoSelectGaussianMixture(
		data,
		defaultAutoMinK,
		maxComponents,
		AutoSelectionCriterionBIC,
		opts...,
	)
}

// Fit estimates model parameters from input samples.
func (gm *GaussianMixture) Fit(data [][]float64) error {
	nSamples, nFeatures, err := numutil.Validate2DFloat64(data, 1)
	if err != nil {
		return err
	}
	if err := gm.validateStaticHyperParams(); err != nil {
		return err
	}
	if err := gm.validateDataDependentHyperParams(nSamples); err != nil {
		return err
	}

	rng := newRNG(gm.seed)
	bestLowerBound := math.Inf(-1)
	var best fitState
	hasBest := false

	for range gm.nInit {
		state, initErr := gm.initializeState(data, rng)
		if initErr != nil {
			return initErr
		}

		lowerBounds := make([]float64, 0, gm.maxIterations)
		prevLowerBound := math.Inf(-1)
		converged := false
		lastIteration := 0

		for iter := 0; iter < gm.maxIterations; iter++ {
			logResp, lowerBound, estErr := gm.expectationStep(
				data,
				state.weights,
				state.means,
				state.covariances,
			)
			if estErr != nil {
				lowerBounds = nil
				break
			}

			lowerBounds = append(lowerBounds, lowerBound)
			state.lowerBound = lowerBound
			lastIteration = iter + 1

			if iter > 0 && math.Abs(lowerBound-prevLowerBound) <= gm.tolerance {
				converged = true
				break
			}
			if iter == gm.maxIterations-1 {
				break
			}
			prevLowerBound = lowerBound

			weights, means, covariances, maxErr := gm.maximizationStep(data, logResp)
			if maxErr != nil {
				lowerBounds = nil
				break
			}
			state.weights = weights
			state.means = means
			state.covariances = covariances
		}

		if len(lowerBounds) == 0 {
			continue
		}

		state.converged = converged
		state.iterations = lastIteration
		state.lowerBounds = slices.Clone(lowerBounds)

		finalLowerBound := lowerBounds[len(lowerBounds)-1]
		if !hasBest || finalLowerBound > bestLowerBound {
			bestLowerBound = finalLowerBound
			best = fitState{
				weights:     slices.Clone(state.weights),
				means:       numutil.Clone2DFloat64(state.means),
				covariances: cloneSymDenseSlice(state.covariances),
				converged:   state.converged,
				iterations:  state.iterations,
				lowerBound:  state.lowerBound,
				lowerBounds: slices.Clone(state.lowerBounds),
			}
			hasBest = true
		}
	}

	if !hasBest {
		return fmt.Errorf("gaussian mixture fitting failed for all initializations")
	}

	normals, err := gm.buildNormals(best.means, best.covariances)
	if err != nil {
		return err
	}

	gm.fitted = true
	gm.nSamples = nSamples
	gm.nFeatures = nFeatures
	gm.weights = best.weights
	gm.means = best.means
	gm.covariances = best.covariances
	gm.normals = normals
	gm.converged = best.converged
	gm.iterations = best.iterations
	gm.lowerBound = best.lowerBound
	gm.lowerBounds = best.lowerBounds
	return nil
}

// FitPredict is equivalent to Fit followed by Predict.
func (gm *GaussianMixture) FitPredict(data [][]float64) ([]int, error) {
	if err := gm.Fit(data); err != nil {
		return nil, err
	}
	return gm.Predict(data)
}

// FitEvaluate is equivalent to Fit followed by Evaluate.
func (gm *GaussianMixture) FitEvaluate(data [][]float64) (Evaluation, error) {
	if err := gm.Fit(data); err != nil {
		return Evaluation{}, err
	}
	return gm.Evaluate(data)
}

// Evaluate returns a consolidated inference summary for the provided samples.
func (gm *GaussianMixture) Evaluate(data [][]float64) (Evaluation, error) {
	logResp, logProbNorm, err := gm.predictLogResponsibilities(data)
	if err != nil {
		return Evaluation{}, err
	}
	if len(logProbNorm) == 0 {
		return Evaluation{}, fmt.Errorf("input data is empty")
	}

	labels := labelsFromLogResp(logResp)

	nSamples := float64(len(logProbNorm))
	averageLogLikelihood := floats.Sum(logProbNorm) / nSamples
	logLikelihood := averageLogLikelihood * nSamples
	nParameters := float64(gm.nParameters())
	bic := -2*logLikelihood + nParameters*math.Log(nSamples)
	aic := -2*logLikelihood + 2*nParameters

	return Evaluation{
		Labels:               labels,
		Weights:              gm.Weights(),
		Means:                gm.Means(),
		LogLikelihood:        logLikelihood,
		AverageLogLikelihood: averageLogLikelihood,
		BIC:                  bic,
		AIC:                  aic,
		Iterations:           gm.Iterations(),
		Converged:            gm.Converged(),
	}, nil
}

// Predict returns the highest posterior component label for each sample.
func (gm *GaussianMixture) Predict(data [][]float64) ([]int, error) {
	logResp, _, err := gm.predictLogResponsibilities(data)
	if err != nil {
		return nil, err
	}

	return labelsFromLogResp(logResp), nil
}

// PredictProba returns posterior component probabilities for each sample.
func (gm *GaussianMixture) PredictProba(data [][]float64) ([][]float64, error) {
	logResp, _, err := gm.predictLogResponsibilities(data)
	if err != nil {
		return nil, err
	}

	probabilities := make([][]float64, len(logResp))
	for sampleIdx, row := range logResp {
		probabilities[sampleIdx] = make([]float64, len(row))
		for componentIdx, value := range row {
			probabilities[sampleIdx][componentIdx] = math.Exp(value)
		}
	}
	return probabilities, nil
}

// ScoreSamples returns per-sample log-likelihood.
func (gm *GaussianMixture) ScoreSamples(data [][]float64) ([]float64, error) {
	_, logProbNorm, err := gm.predictLogResponsibilities(data)
	if err != nil {
		return nil, err
	}
	return logProbNorm, nil
}

// Score returns the average log-likelihood across all samples.
func (gm *GaussianMixture) Score(data [][]float64) (float64, error) {
	logProbNorm, err := gm.ScoreSamples(data)
	if err != nil {
		return 0, err
	}
	if len(logProbNorm) == 0 {
		return 0, fmt.Errorf("input data is empty")
	}
	return floats.Sum(logProbNorm) / float64(len(logProbNorm)), nil
}

// BIC returns the Bayesian Information Criterion for the input samples.
func (gm *GaussianMixture) BIC(data [][]float64) (float64, error) {
	score, err := gm.Score(data)
	if err != nil {
		return 0, err
	}
	nSamples := float64(len(data))
	return -2*score*nSamples + float64(gm.nParameters())*math.Log(nSamples), nil
}

// AIC returns the Akaike Information Criterion for the input samples.
func (gm *GaussianMixture) AIC(data [][]float64) (float64, error) {
	score, err := gm.Score(data)
	if err != nil {
		return 0, err
	}
	nSamples := float64(len(data))
	return -2*score*nSamples + 2*float64(gm.nParameters()), nil
}

// Weights returns a copy of component weights.
func (gm *GaussianMixture) Weights() []float64 {
	return slices.Clone(gm.weights)
}

// Means returns a copy of component means.
func (gm *GaussianMixture) Means() [][]float64 {
	return numutil.Clone2DFloat64(gm.means)
}

// Covariances returns a copy of component covariance matrices.
func (gm *GaussianMixture) Covariances() [][][]float64 {
	result := make([][][]float64, len(gm.covariances))
	for componentIdx, covariance := range gm.covariances {
		result[componentIdx] = numutil.DenseToSlice(covariance)
	}
	return result
}

// Converged reports whether the last fit reached the tolerance criterion.
func (gm *GaussianMixture) Converged() bool {
	return gm.converged
}

// Iterations returns the number of EM iterations in the best initialization.
func (gm *GaussianMixture) Iterations() int {
	return gm.iterations
}

// LowerBound returns the average per-sample lower bound from the final E-step.
func (gm *GaussianMixture) LowerBound() float64 {
	return gm.lowerBound
}

// LowerBounds returns all intermediate lower bound values.
func (gm *GaussianMixture) LowerBounds() []float64 {
	return slices.Clone(gm.lowerBounds)
}

func (gm *GaussianMixture) validateStaticHyperParams() error {
	if gm.nComponents <= 0 {
		return fmt.Errorf("n_components must be positive, got %d", gm.nComponents)
	}
	if gm.maxIterations <= 0 {
		return fmt.Errorf("max_iterations must be positive, got %d", gm.maxIterations)
	}
	if gm.tolerance < 0 {
		return fmt.Errorf("tolerance must be non-negative, got %.6g", gm.tolerance)
	}
	if gm.regularization < 0 {
		return fmt.Errorf("regularization must be non-negative, got %.6g", gm.regularization)
	}
	if gm.nInit <= 0 {
		return fmt.Errorf("n_init must be positive, got %d", gm.nInit)
	}
	if gm.initParams != InitParamsKMeans && gm.initParams != InitParamsRandom {
		return fmt.Errorf("unsupported init_params=%q", gm.initParams)
	}
	return nil
}

func (gm *GaussianMixture) validateDataDependentHyperParams(nSamples int) error {
	if gm.nComponents > nSamples {
		return fmt.Errorf("n_components=%d exceeds sample count=%d", gm.nComponents, nSamples)
	}
	return nil
}

func validateAutoSelectionRange(minComponents, maxComponents, nSamples int) error {
	if minComponents <= 0 {
		return fmt.Errorf("min_components must be positive, got %d", minComponents)
	}
	if maxComponents < minComponents {
		return fmt.Errorf("max_components must be >= min_components, got min=%d max=%d", minComponents, maxComponents)
	}
	if maxComponents > nSamples {
		return fmt.Errorf("max_components=%d exceeds sample count=%d", maxComponents, nSamples)
	}
	return nil
}

func validateAutoSelectionCriterion(criterion AutoSelectionCriterion) error {
	if criterion != AutoSelectionCriterionBIC && criterion != AutoSelectionCriterionAIC {
		return fmt.Errorf("unsupported selection criterion=%q", criterion)
	}
	return nil
}

func selectScoreByCriterion(evaluation Evaluation, criterion AutoSelectionCriterion) float64 {
	switch criterion {
	case AutoSelectionCriterionAIC:
		return evaluation.AIC
	case AutoSelectionCriterionBIC:
		return evaluation.BIC
	default:
		return math.Inf(1)
	}
}

func cloneEvaluation(e Evaluation) Evaluation {
	cloned := e
	cloned.Labels = slices.Clone(e.Labels)
	cloned.Weights = slices.Clone(e.Weights)
	cloned.Means = numutil.Clone2DFloat64(e.Means)
	return cloned
}

func (gm *GaussianMixture) initializeState(data [][]float64, rng *rand.Rand) (fitState, error) {
	resp, err := gm.initializeResponsibilities(data, rng)
	if err != nil {
		return fitState{}, err
	}
	weights, means, covariances, err := gm.estimateGaussianParameters(data, resp)
	if err != nil {
		return fitState{}, err
	}
	return fitState{
		weights:     weights,
		means:       means,
		covariances: covariances,
	}, nil
}

func (gm *GaussianMixture) initializeResponsibilities(data [][]float64, rng *rand.Rand) ([][]float64, error) {
	nSamples := len(data)
	resp := make([][]float64, nSamples)
	for sampleIdx := range nSamples {
		resp[sampleIdx] = make([]float64, gm.nComponents)
	}

	switch gm.initParams {
	case InitParamsRandom:
		for sampleIdx := range nSamples {
			rowTotal := 0.0
			for componentIdx := range gm.nComponents {
				value := rng.Float64() + minComponentMass
				resp[sampleIdx][componentIdx] = value
				rowTotal += value
			}
			floats.Scale(1.0/rowTotal, resp[sampleIdx])
		}
		return resp, nil

	case InitParamsKMeans:
		centers := kmeansPlusPlus(data, gm.nComponents, rng)
		for sampleIdx, sample := range data {
			bestComponent := 0
			bestDistance := squaredDistance(sample, centers[0])
			for componentIdx := 1; componentIdx < gm.nComponents; componentIdx++ {
				currentDistance := squaredDistance(sample, centers[componentIdx])
				if currentDistance < bestDistance {
					bestDistance = currentDistance
					bestComponent = componentIdx
				}
			}

			if gm.nComponents == 1 {
				resp[sampleIdx][0] = 1
				continue
			}

			offDiagonalValue := initSmoothing / float64(gm.nComponents-1)
			for componentIdx := range gm.nComponents {
				resp[sampleIdx][componentIdx] = offDiagonalValue
			}
			resp[sampleIdx][bestComponent] = 1 - initSmoothing
		}
		return resp, nil
	}

	return nil, fmt.Errorf("unsupported init_params=%q", gm.initParams)
}

func (gm *GaussianMixture) expectationStep(
	data [][]float64,
	weights []float64,
	means [][]float64,
	covariances []*mat.SymDense,
) (logResp [][]float64, lowerBound float64, err error) {
	normals, err := gm.buildNormals(means, covariances)
	if err != nil {
		return nil, 0, err
	}
	logResp, logProbNorm, err := gm.estimateLogResponsibilities(data, weights, normals)
	if err != nil {
		return nil, 0, err
	}
	return logResp, floats.Sum(logProbNorm) / float64(len(logProbNorm)), nil
}

func (gm *GaussianMixture) buildNormals(means [][]float64, covariances []*mat.SymDense) ([]*distmv.Normal, error) {
	normals := make([]*distmv.Normal, gm.nComponents)
	for componentIdx := range gm.nComponents {
		mean := slices.Clone(means[componentIdx])
		covariance := cloneSymDense(covariances[componentIdx])
		normal, ok := distmv.NewNormal(mean, covariance, nil)
		if !ok {
			return nil, fmt.Errorf("component %d covariance is not positive definite", componentIdx)
		}
		normals[componentIdx] = normal
	}
	return normals, nil
}

func (gm *GaussianMixture) estimateLogResponsibilities(
	data [][]float64,
	weights []float64,
	normals []*distmv.Normal,
) ([][]float64, []float64, error) {
	nSamples := len(data)
	logResp := make([][]float64, nSamples)
	logRespFlat := make([]float64, nSamples*gm.nComponents)
	logProbNorm := make([]float64, nSamples)
	weightedLogProb := make([]float64, gm.nComponents)

	for sampleIdx, sample := range data {
		for componentIdx := range gm.nComponents {
			weight := math.Max(weights[componentIdx], minComponentMass)
			weightedLogProb[componentIdx] = math.Log(weight) + normals[componentIdx].LogProb(sample)
		}

		logNorm := logSumExp(weightedLogProb)
		logProbNorm[sampleIdx] = logNorm
		row := logRespFlat[sampleIdx*gm.nComponents : (sampleIdx+1)*gm.nComponents]
		copy(row, weightedLogProb)
		floats.AddConst(-logNorm, row)
		logResp[sampleIdx] = row
	}

	return logResp, logProbNorm, nil
}

func (gm *GaussianMixture) maximizationStep(
	data [][]float64,
	logResp [][]float64,
) (weights []float64, means [][]float64, covariances []*mat.SymDense, err error) {
	nSamples := len(data)
	resp := make([][]float64, nSamples)
	respFlat := make([]float64, nSamples*gm.nComponents)
	for sampleIdx := range nSamples {
		row := respFlat[sampleIdx*gm.nComponents : (sampleIdx+1)*gm.nComponents]
		resp[sampleIdx] = row
		for componentIdx := range gm.nComponents {
			row[componentIdx] = math.Exp(logResp[sampleIdx][componentIdx])
		}
	}
	return gm.estimateGaussianParameters(data, resp)
}

func (gm *GaussianMixture) estimateGaussianParameters(
	data [][]float64,
	resp [][]float64,
) (weights []float64, means [][]float64, covariances []*mat.SymDense, err error) {
	nSamples := len(data)
	nFeatures := len(data[0])

	nk := make([]float64, gm.nComponents)
	for componentIdx := range gm.nComponents {
		componentMass := 0.0
		for sampleIdx := range nSamples {
			componentMass += resp[sampleIdx][componentIdx]
		}
		nk[componentIdx] = componentMass + minComponentMass
	}

	weights = make([]float64, gm.nComponents)
	means = make([][]float64, gm.nComponents)
	for componentIdx := range gm.nComponents {
		weights[componentIdx] = nk[componentIdx] / float64(nSamples)
		means[componentIdx] = make([]float64, nFeatures)
	}

	for sampleIdx, sample := range data {
		for componentIdx := range gm.nComponents {
			responsibility := resp[sampleIdx][componentIdx]
			floats.AddScaled(means[componentIdx], responsibility, sample)
		}
	}
	for componentIdx := range gm.nComponents {
		scale := 1.0 / nk[componentIdx]
		floats.Scale(scale, means[componentIdx])
	}

	covariances = make([]*mat.SymDense, gm.nComponents)
	weightedCenteredData := make([]float64, nSamples*nFeatures)
	weightedCentered := mat.NewDense(nSamples, nFeatures, weightedCenteredData)
	gram := mat.NewDense(nFeatures, nFeatures, nil)
	for componentIdx := range gm.nComponents {
		componentMean := means[componentIdx]
		for sampleIdx, sample := range data {
			row := weightedCenteredData[sampleIdx*nFeatures : (sampleIdx+1)*nFeatures]
			copy(row, sample)
			floats.AddScaled(row, -1.0, componentMean)
			responsibility := resp[sampleIdx][componentIdx]
			if responsibility <= 0 {
				clear(row)
				continue
			}
			floats.Scale(math.Sqrt(responsibility), row)
		}

		gram.Mul(weightedCentered.T(), weightedCentered)
		covariance := mat.NewSymDense(nFeatures, nil)
		scale := 1.0 / nk[componentIdx]
		for row := range nFeatures {
			for col := 0; col <= row; col++ {
				value := gram.At(row, col) * scale
				if row == col {
					value += gm.regularization
				}
				covariance.SetSym(row, col, value)
			}
		}
		covariances[componentIdx] = covariance
	}

	return weights, means, covariances, nil
}

func (gm *GaussianMixture) predictLogResponsibilities(data [][]float64) ([][]float64, []float64, error) {
	if err := gm.ensurePredictable(data); err != nil {
		return nil, nil, err
	}
	return gm.estimateLogResponsibilities(data, gm.weights, gm.normals)
}

func (gm *GaussianMixture) ensurePredictable(data [][]float64) error {
	if !gm.fitted {
		return fmt.Errorf("gaussian mixture model is not fitted")
	}
	return numutil.ValidateRowsWithFeatureCount(data, gm.nFeatures)
}

func (gm *GaussianMixture) nParameters() int {
	covarianceParams := gm.nComponents * gm.nFeatures * (gm.nFeatures + 1) / 2
	meanParams := gm.nComponents * gm.nFeatures
	return covarianceParams + meanParams + gm.nComponents - 1
}

func argmaxIndex(values []float64) int {
	if len(values) == 0 {
		return 0
	}
	return floats.MaxIdx(values)
}

func labelsFromLogResp(logResp [][]float64) []int {
	labels := make([]int, len(logResp))
	for sampleIdx, row := range logResp {
		labels[sampleIdx] = argmaxIndex(row)
	}
	return labels
}

func kmeansPlusPlus(data [][]float64, nCenters int, rng *rand.Rand) [][]float64 {
	nSamples := len(data)
	centers := make([][]float64, 0, nCenters)
	firstCenterIdx := rng.IntN(nSamples)
	centers = append(centers, slices.Clone(data[firstCenterIdx]))

	minDistances := make([]float64, nSamples)
	for sampleIdx := range nSamples {
		minDistances[sampleIdx] = squaredDistance(data[sampleIdx], centers[0])
	}

	for len(centers) < nCenters {
		totalDistance := floats.Sum(minDistances)
		if totalDistance <= 0 {
			centers = append(centers, slices.Clone(data[rng.IntN(nSamples)]))
			continue
		}

		threshold := rng.Float64() * totalDistance
		accumulator := 0.0
		nextCenterIdx := nSamples - 1
		for sampleIdx, distance := range minDistances {
			accumulator += distance
			if accumulator >= threshold {
				nextCenterIdx = sampleIdx
				break
			}
		}
		centers = append(centers, slices.Clone(data[nextCenterIdx]))

		lastCenter := centers[len(centers)-1]
		for sampleIdx := range nSamples {
			distance := squaredDistance(data[sampleIdx], lastCenter)
			if distance < minDistances[sampleIdx] {
				minDistances[sampleIdx] = distance
			}
		}
	}

	return centers
}

func squaredDistance(a, b []float64) float64 {
	distance := 0.0
	for idx := range a {
		diff := a[idx] - b[idx]
		distance += diff * diff
	}
	return distance
}

func logSumExp(values []float64) float64 {
	maxValue := floats.Max(values)

	accumulator := 0.0
	for _, value := range values {
		accumulator += math.Exp(value - maxValue)
	}
	return maxValue + math.Log(accumulator)
}

func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

func cloneSymDenseSlice(covariances []*mat.SymDense) []*mat.SymDense {
	result := make([]*mat.SymDense, len(covariances))
	for idx, covariance := range covariances {
		result[idx] = cloneSymDense(covariance)
	}
	return result
}

func cloneSymDense(covariance *mat.SymDense) *mat.SymDense {
	nFeatures := covariance.SymmetricDim()
	result := mat.NewSymDense(nFeatures, nil)
	result.CopySym(covariance)
	return result
}

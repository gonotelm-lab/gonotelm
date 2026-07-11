package manifold

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/decomposition"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

const (
	machineEpsilon      = 2.220446049250313e-16
	defaultPerplexity   = 30.0
	defaultEarlyExag    = 12.0
	defaultMaxIter      = 1000
	defaultNIterNoProg  = 300
	defaultMinGradNorm  = 1e-7
	defaultMinGain      = 0.01
	defaultNIterCheck   = 50
	defaultExploreIters = 250

	defaultGoMLXBackendConfig = "go"
)

// InitMethod controls the initialization strategy for embeddings.
type InitMethod string

const (
	// InitPCA uses PCA projection as initialization.
	InitPCA InitMethod = "pca"
	// InitRandom uses Gaussian random initialization (std=1e-4).
	InitRandom InitMethod = "random"
)

// TSNEOption configures a TSNE model.
type TSNEOption func(*TSNE)

// WithPerplexity sets perplexity (default: 30).
//
// Range:
// - Hard constraint: (0, n_samples)
// - Practical tuning: [5, 50] for most visualization tasks
//
// Tuning guide:
// - Smaller (5-20): emphasize fine local neighborhoods, easier to split tiny clusters.
// - Larger (30-100): smoother global layout, needs more samples to stay stable.
// - Rule of thumb: keep perplexity < n_samples.
func WithPerplexity(perplexity float64) TSNEOption {
	return func(t *TSNE) {
		t.perplexity = perplexity
	}
}

// WithEarlyExaggeration sets early exaggeration factor (default: 12).
//
// Range:
// - Hard constraint: [1, +inf)
// - Practical tuning: [4, 32]
//
// Tuning guide:
// - Larger values usually increase early cluster separation.
// - Too large can create over-separated islands or unstable optimization.
func WithEarlyExaggeration(value float64) TSNEOption {
	return func(t *TSNE) {
		t.earlyExaggeration = value
	}
}

// WithLearningRate sets a fixed learning rate and disables auto mode.
//
// Range:
// - Hard constraint: (0, +inf)
// - Practical tuning: [10, 1000]
//
// Tuning guide:
// - Too low: optimization is slow and can get stuck.
// - Too high: points may form a "ball" or diverge.
// - Prefer WithAutoLearningRate unless you need strict reproducibility.
func WithLearningRate(value float64) TSNEOption {
	return func(t *TSNE) {
		t.learningRate = value
		t.learningRateAuto = false
	}
}

// WithAutoLearningRate enables sklearn-compatible auto learning rate.
//
// Effective value is max(n_samples / early_exaggeration / 4, 50).
//
// Range:
// - No numeric input; this switches to automatic strategy.
func WithAutoLearningRate() TSNEOption {
	return func(t *TSNE) {
		t.learningRateAuto = true
	}
}

// WithMaxIter sets total optimization iterations (default: 1000).
//
// Range:
// - Hard constraint: integer >= 250 (exploration phase length)
// - Practical tuning: [500, 5000]
//
// Tuning guide:
// - Increase when KL divergence is still dropping at the end.
// - Reduce for quick previews on large data.
func WithMaxIter(value int) TSNEOption {
	return func(t *TSNE) {
		t.maxIter = value
	}
}

// WithNIterWithoutProgress sets early-stop patience after exploration stage (default: 300).
//
// Range:
// - Hard constraint: integer >= 0
// - Practical tuning: [50, 1000]
//
// Larger values make stopping more conservative, reducing premature stop risk.
func WithNIterWithoutProgress(value int) TSNEOption {
	return func(t *TSNE) {
		t.nIterWithoutProgress = value
	}
}

// WithMinGradNorm sets the minimum gradient norm threshold (default: 1e-7).
//
// Range:
// - Hard constraint: [0, +inf)
// - Practical tuning: [1e-9, 1e-5]
//
// Larger threshold stops earlier; smaller threshold runs longer for finer convergence.
func WithMinGradNorm(value float64) TSNEOption {
	return func(t *TSNE) {
		t.minGradNorm = value
	}
}

// WithInitMethod sets embedding initialization method (default: InitPCA).
//
// Range:
// - Hard constraint: InitPCA or InitRandom
//
// - InitPCA: usually more stable and converges faster on structured data.
// - InitRandom: useful to test layout robustness across seeds.
func WithInitMethod(method InitMethod) TSNEOption {
	return func(t *TSNE) {
		t.init = method
	}
}

// WithRandomSeed sets random seed (used by InitRandom path).
//
// Range:
// - Hard constraint: any int64
//
// Keep this fixed to make result comparison repeatable.
func WithRandomSeed(seed int64) TSNEOption {
	return func(t *TSNE) {
		t.randomSeed = seed
	}
}

// WithGoMLXCore enables or disables GoMLX-based KL+gradient kernel.
//
// Range:
// - boolean: true/false
//
// Enable when you want potentially faster kernel execution through GoMLX backend.
func WithGoMLXCore(enabled bool) TSNEOption {
	return func(t *TSNE) {
		t.useGoMLXCore = enabled
	}
}

// WithGoMLXBackendConfig sets the backend config used by GoMLX core.
//
// Range:
// - Hard constraint (when GoMLX core enabled): non-empty string
//
// Example values:
// - "go"
// - "xla:cpu"
// - "xla:cuda"
func WithGoMLXBackendConfig(config string) TSNEOption {
	return func(t *TSNE) {
		t.goMLXBackendConfig = config
	}
}

// TSNE implements exact t-SNE optimization (O(N^2)).
//
// This implementation aligns with sklearn's core behavior:
// - Per-row binary search for perplexity in high-dimensional conditional probs
// - Symmetrized and normalized joint probabilities P
// - Two-stage optimization (early exaggeration then normal) with momentum/gains
type TSNE struct {
	nComponents          int
	perplexity           float64
	earlyExaggeration    float64
	learningRate         float64
	learningRateAuto     bool
	maxIter              int
	nIterWithoutProgress int
	minGradNorm          float64
	init                 InitMethod
	randomSeed           int64

	nIterCheck      int
	explorationIter int
	minGain         float64

	fitted        bool
	embedding     [][]float64
	klDivergence  float64
	nIter         int
	learningRate_ float64

	useGoMLXCore       bool
	goMLXBackendConfig string
}

// NewTSNE creates a t-SNE model.
//
// nComponents means target embedding dimension after dimensionality reduction:
// - 2 for standard 2D visualization
// - 3 for 3D visualization
// - Hard constraint: integer >= 1
//
// sklearn-like defaults:
// perplexity=30, early_exaggeration=12, auto learning-rate, max_iter=1000.
//
// Example:
//
//	model, err := NewTSNE(
//		2,
//		WithPerplexity(30),
//		WithMaxIter(1200),
//		WithAutoLearningRate(),
//		WithInitMethod(InitPCA),
//	)
//	if err != nil {
//		return err
//	}
func NewTSNE(nComponents int, opts ...TSNEOption) (*TSNE, error) {
	model := &TSNE{
		nComponents:          nComponents,
		perplexity:           defaultPerplexity,
		earlyExaggeration:    defaultEarlyExag,
		learningRateAuto:     true,
		maxIter:              defaultMaxIter,
		nIterWithoutProgress: defaultNIterNoProg,
		minGradNorm:          defaultMinGradNorm,
		init:                 InitPCA,
		randomSeed:           0,
		nIterCheck:           defaultNIterCheck,
		explorationIter:      defaultExploreIters,
		minGain:              defaultMinGain,
		useGoMLXCore:         false,
		goMLXBackendConfig:   defaultGoMLXBackendConfig,
	}

	for _, opt := range opts {
		opt(model)
	}
	if err := model.validateStaticConfig(); err != nil {
		return nil, fmt.Errorf("invalid TSNE config: %w", err)
	}

	return model, nil
}

// Fit runs t-SNE and stores the embedding.
func (t *TSNE) Fit(data [][]float64) error {
	embedding, err := t.FitTransform(data)
	if err != nil {
		return err
	}
	t.embedding = embedding
	return nil
}

// FitTransform runs t-SNE and returns embedded coordinates.
func (t *TSNE) FitTransform(data [][]float64) ([][]float64, error) {
	nSamples, _, err := validateTSNEInput(data)
	if err != nil {
		return nil, err
	}
	if err := t.validateDataDependentConfig(nSamples); err != nil {
		return nil, err
	}

	if t.learningRateAuto {
		t.learningRate_ = math.Max(float64(nSamples)/t.earlyExaggeration/4.0, 50.0)
	} else {
		t.learningRate_ = t.learningRate
	}

	distances := pairwiseSquaredDistances(data)
	jointProbabilities, err := jointProbabilities(distances, nSamples, t.perplexity)
	if err != nil {
		return nil, err
	}

	initialEmbedding, err := t.initializeEmbedding(data, nSamples)
	if err != nil {
		return nil, err
	}

	dof := max(t.nComponents-1, 1)
	params := numutil.Flatten2DFloat64(initialEmbedding)
	workspace := newKLDivergenceWorkspace(nSamples, t.nComponents)

	phase1P := slices.Clone(jointProbabilities)
	for i := range phase1P {
		phase1P[i] *= t.earlyExaggeration
	}

	phase1Opts := gradientDescentOptions{
		startIter:             0,
		maxIter:               t.explorationIter,
		nIterCheck:            t.nIterCheck,
		nIterWithoutProgress:  t.explorationIter,
		momentum:              0.5,
		learningRate:          t.learningRate_,
		minGain:               t.minGain,
		minGradNorm:           t.minGradNorm,
		computeObjectiveValue: true,
	}
	phase1Objective, phase1Cleanup := t.makeObjective(phase1P, dof, nSamples, workspace)
	defer phase1Cleanup()

	params, _, iteration := gradientDescent(
		params,
		phase1Opts,
		phase1Objective,
	)

	remaining := t.maxIter - t.explorationIter
	phase2Objective, phase2Cleanup := t.makeObjective(jointProbabilities, dof, nSamples, workspace)
	defer phase2Cleanup()

	if iteration < t.explorationIter || remaining > 0 {
		phase2Opts := gradientDescentOptions{
			startIter:             iteration + 1,
			maxIter:               t.maxIter,
			nIterCheck:            t.nIterCheck,
			nIterWithoutProgress:  t.nIterWithoutProgress,
			momentum:              0.8,
			learningRate:          t.learningRate_,
			minGain:               t.minGain,
			minGradNorm:           t.minGradNorm,
			computeObjectiveValue: true,
		}
		params, _, iteration = gradientDescent(
			params,
			phase2Opts,
			phase2Objective,
		)
	}

	embedding := numutil.ReshapeRowMajorFloat64(params, nSamples, t.nComponents)
	finalKL, _ := phase2Objective(params, true)
	if math.IsNaN(finalKL) || math.IsInf(finalKL, 0) {
		return nil, fmt.Errorf("tsne optimization diverged, invalid kl divergence: %v", finalKL)
	}
	t.embedding = embedding
	t.klDivergence = finalKL
	t.nIter = iteration
	t.fitted = true
	return embedding, nil
}

// Embedding returns fitted embedding.
func (t *TSNE) Embedding() [][]float64 {
	return numutil.Clone2DFloat64(t.embedding)
}

// KLDivergence returns final KL divergence.
func (t *TSNE) KLDivergence() float64 {
	return t.klDivergence
}

// NIter returns the final iteration index.
func (t *TSNE) NIter() int {
	return t.nIter
}

// LearningRate returns the effective learning rate.
func (t *TSNE) LearningRate() float64 {
	return t.learningRate_
}

func (t *TSNE) validateStaticConfig() error {
	if t.nComponents <= 0 {
		return fmt.Errorf("n_components must be positive, got %d", t.nComponents)
	}
	if t.perplexity <= 0 {
		return fmt.Errorf("perplexity must be positive, got %.6f", t.perplexity)
	}
	if t.earlyExaggeration < 1 {
		return fmt.Errorf("early_exaggeration must be >= 1, got %.6f", t.earlyExaggeration)
	}
	if !t.learningRateAuto && t.learningRate <= 0 {
		return fmt.Errorf("learning_rate must be positive, got %.6f", t.learningRate)
	}
	if t.maxIter < t.explorationIter {
		return fmt.Errorf("max_iter must be >= %d, got %d", t.explorationIter, t.maxIter)
	}
	if t.nIterWithoutProgress < 0 {
		return fmt.Errorf("n_iter_without_progress must be >= 0, got %d", t.nIterWithoutProgress)
	}
	if t.minGradNorm < 0 {
		return fmt.Errorf("min_grad_norm must be >= 0, got %.6f", t.minGradNorm)
	}
	if t.nIterCheck <= 0 {
		return fmt.Errorf("n_iter_check must be > 0, got %d", t.nIterCheck)
	}
	if t.init != InitPCA && t.init != InitRandom {
		return fmt.Errorf("unsupported init method: %q", t.init)
	}
	if t.useGoMLXCore && strings.TrimSpace(t.goMLXBackendConfig) == "" {
		return fmt.Errorf("gomlx backend config must not be empty when gomlx core is enabled")
	}
	return nil
}

func (t *TSNE) validateDataDependentConfig(nSamples int) error {
	if t.perplexity >= float64(nSamples) {
		return fmt.Errorf("perplexity %.6f must be less than n_samples %d", t.perplexity, nSamples)
	}
	return nil
}

func (t *TSNE) initializeEmbedding(data [][]float64, nSamples int) ([][]float64, error) {
	switch t.init {
	case InitPCA:
		pca := decomposition.NewPCA(t.nComponents)
		projected, err := pca.FitTransform(data)
		if err != nil {
			return nil, fmt.Errorf("pca init failed: %w", err)
		}

		scaleStd := stat.PopStdDev(numutil.ColumnFrom2DFloat64(projected, 0), nil)
		if scaleStd < machineEpsilon {
			scaleStd = 1
		}
		scale := 1e-4 / scaleStd
		for i := range projected {
			floats.Scale(scale, projected[i])
		}
		return projected, nil

	case InitRandom:
		generator := rand.New(rand.NewSource(t.randomSeed))
		embedding := make([][]float64, nSamples)
		for i := 0; i < nSamples; i++ {
			embedding[i] = make([]float64, t.nComponents)
			for j := 0; j < t.nComponents; j++ {
				embedding[i][j] = 1e-4 * generator.NormFloat64()
			}
		}
		return embedding, nil

	default:
		return nil, fmt.Errorf("unsupported init method: %q", t.init)
	}
}

func validateTSNEInput(data [][]float64) (int, int, error) {
	if len(data) < 2 {
		return 0, 0, fmt.Errorf("tsne requires at least 2 samples, got %d", len(data))
	}
	nFeatures := len(data[0])
	if nFeatures == 0 {
		return 0, 0, fmt.Errorf("input data has zero features")
	}
	for i, row := range data {
		if len(row) != nFeatures {
			return 0, 0, fmt.Errorf("row %d has %d features, expected %d", i, len(row), nFeatures)
		}
		for j, value := range row {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return 0, 0, fmt.Errorf("input contains invalid value at row %d col %d", i, j)
			}
		}
	}
	return len(data), nFeatures, nil
}

func pairwiseSquaredDistances(data [][]float64) []float64 {
	nSamples := len(data)
	rowNorms := make([]float64, nSamples)
	for i := 0; i < nSamples; i++ {
		rowNorms[i] = floats.Dot(data[i], data[i])
	}

	distances := make([]float64, nSamples*nSamples)

	for i := 0; i < nSamples; i++ {
		rowI := data[i]
		for j := i + 1; j < nSamples; j++ {
			distance := rowNorms[i] + rowNorms[j] - 2.0*floats.Dot(rowI, data[j])
			if distance < 0 && distance > -1e-12 {
				distance = 0
			}
			distances[i*nSamples+j] = distance
			distances[j*nSamples+i] = distance
		}
	}
	return distances
}

func jointProbabilities(distances []float64, nSamples int, perplexity float64) ([]float64, error) {
	conditional := make([]float64, nSamples*nSamples)
	logPerplexity := math.Log(perplexity)

	for i := 0; i < nSamples; i++ {
		if err := binarySearchPerplexityRow(distances, conditional, nSamples, i, logPerplexity); err != nil {
			return nil, err
		}
	}

	joint := make([]float64, nSamples*nSamples)
	sumJoint := 0.0
	for i := 0; i < nSamples; i++ {
		for j := i + 1; j < nSamples; j++ {
			value := conditional[i*nSamples+j] + conditional[j*nSamples+i]
			joint[i*nSamples+j] = value
			joint[j*nSamples+i] = value
			sumJoint += 2 * value
		}
	}
	if sumJoint < machineEpsilon {
		return nil, fmt.Errorf("joint probability normalization underflow")
	}
	floats.Scale(1.0/sumJoint, joint)

	for i := 0; i < nSamples; i++ {
		for j := i + 1; j < nSamples; j++ {
			value := joint[i*nSamples+j]
			if value < machineEpsilon {
				value = machineEpsilon
			}
			joint[i*nSamples+j] = value
			joint[j*nSamples+i] = value
		}
	}
	return joint, nil
}

func binarySearchPerplexityRow(
	distances []float64,
	conditional []float64,
	nSamples int,
	row int,
	targetEntropy float64,
) error {
	const (
		maxBinarySearchSteps = 50
		entropyTolerance     = 1e-5
	)

	beta := 1.0
	betaMin := math.Inf(-1)
	betaMax := math.Inf(1)
	rowOffset := row * nSamples
	rowConditional := conditional[rowOffset : rowOffset+nSamples]
	rowDistances := distances[rowOffset : rowOffset+nSamples]

	for step := 0; step < maxBinarySearchSteps; step++ {
		for col := 0; col < nSamples; col++ {
			if col == row {
				rowConditional[col] = 0
				continue
			}
			rowConditional[col] = math.Exp(-rowDistances[col] * beta)
		}
		sumP := floats.Sum(rowConditional)

		if sumP < machineEpsilon {
			uniform := 1.0 / float64(nSamples-1)
			for col := 0; col < nSamples; col++ {
				if col == row {
					rowConditional[col] = 0
				} else {
					rowConditional[col] = uniform
				}
			}
			return nil
		}

		floats.Scale(1.0/sumP, rowConditional)
		rowConditional[row] = 0
		entropy := 0.0
		for col := 0; col < nSamples; col++ {
			if col == row {
				continue
			}
			value := rowConditional[col]
			if value > 0 {
				entropy -= value * math.Log(value)
			}
		}

		diff := entropy - targetEntropy
		if math.Abs(diff) <= entropyTolerance {
			return nil
		}

		if diff > 0 {
			betaMin = beta
			if math.IsInf(betaMax, 1) {
				beta *= 2
			} else {
				beta = (beta + betaMax) / 2
			}
		} else {
			betaMax = beta
			if math.IsInf(betaMin, -1) {
				beta /= 2
			} else {
				beta = (beta + betaMin) / 2
			}
		}
	}

	for col := 0; col < nSamples; col++ {
		if col == row {
			rowConditional[col] = 0
		}
	}
	return nil
}

func (t *TSNE) makeObjective(
	jointProbabilities []float64,
	degreesOfFreedom int,
	nSamples int,
	workspace *klDivergenceWorkspace,
) (func([]float64, bool) (float64, []float64), func()) {
	pureObjective := func(params []float64, computeError bool) (float64, []float64) {
		return klDivergenceExact(
			params,
			jointProbabilities,
			degreesOfFreedom,
			nSamples,
			t.nComponents,
			computeError,
			workspace,
		)
	}

	if !t.useGoMLXCore {
		return pureObjective, func() {}
	}

	kernel, err := newGoMLXKLDivergenceKernel(
		jointProbabilities,
		degreesOfFreedom,
		nSamples,
		t.nComponents,
		t.goMLXBackendConfig,
	)
	if err != nil {
		return pureObjective, func() {}
	}

	objective := func(params []float64, computeError bool) (float64, []float64) {
		kl, grad, evalErr := kernel.Evaluate(params)
		if evalErr != nil {
			return pureObjective(params, computeError)
		}
		if !computeError {
			kl = math.NaN()
		}
		return kl, grad
	}
	return objective, kernel.Close
}

func klDivergenceExact(
	params []float64,
	jointProbabilities []float64,
	degreesOfFreedom int,
	nSamples int,
	nComponents int,
	computeError bool,
	workspace *klDivergenceWorkspace,
) (float64, []float64) {
	if workspace == nil {
		workspace = newKLDivergenceWorkspace(nSamples, nComponents)
	}
	workspace.reset()
	num := workspace.num
	q := workspace.q
	grad := workspace.grad
	rowNorms := workspace.rowNorms

	doff := float64(degreesOfFreedom)
	power := -((doff + 1.0) / 2.0)

	for i := 0; i < nSamples; i++ {
		baseI := i * nComponents
		rowI := params[baseI : baseI+nComponents]
		rowNorms[i] = floats.Dot(rowI, rowI)
	}

	sumNum := 0.0
	for i := 0; i < nSamples; i++ {
		rowOffsetI := i * nSamples
		baseI := i * nComponents
		rowI := params[baseI : baseI+nComponents]
		for j := i + 1; j < nSamples; j++ {
			baseJ := j * nComponents
			rowJ := params[baseJ : baseJ+nComponents]
			squaredDistance := rowNorms[i] + rowNorms[j] - 2.0*floats.Dot(rowI, rowJ)
			if squaredDistance < 0 && squaredDistance > -1e-12 {
				squaredDistance = 0
			}
			value := math.Pow(1.0+squaredDistance/doff, power)
			num[rowOffsetI+j] = value
			num[j*nSamples+i] = value
			sumNum += 2 * value
		}
	}
	if sumNum < machineEpsilon {
		sumNum = machineEpsilon
	}

	for i := 0; i < nSamples; i++ {
		rowOffsetI := i * nSamples
		for j := i + 1; j < nSamples; j++ {
			value := num[rowOffsetI+j] / sumNum
			if value < machineEpsilon {
				value = machineEpsilon
			}
			q[rowOffsetI+j] = value
			q[j*nSamples+i] = value
		}
	}

	kl := math.NaN()
	if computeError {
		kl = 0.0
		for i := 0; i < nSamples; i++ {
			rowOffsetI := i * nSamples
			for j := i + 1; j < nSamples; j++ {
				pij := jointProbabilities[rowOffsetI+j]
				if pij < machineEpsilon {
					pij = machineEpsilon
				}
				qij := q[rowOffsetI+j]
				kl += 2.0 * pij * math.Log(pij/qij)
			}
		}
	}

	for i := 0; i < nSamples; i++ {
		rowOffsetI := i * nSamples
		baseI := i * nComponents
		rowI := params[baseI : baseI+nComponents]
		gradI := grad[baseI : baseI+nComponents]
		for j := 0; j < nSamples; j++ {
			if i == j {
				continue
			}
			pq := (jointProbabilities[rowOffsetI+j] - q[rowOffsetI+j]) * num[rowOffsetI+j]
			baseJ := j * nComponents
			rowJ := params[baseJ : baseJ+nComponents]

			floats.AddScaled(gradI, pq, rowI)
			floats.AddScaled(gradI, -pq, rowJ)
		}
	}

	constant := 2.0 * (doff + 1.0) / doff
	floats.Scale(constant, grad)

	return kl, grad
}

type klDivergenceWorkspace struct {
	num      []float64
	q        []float64
	grad     []float64
	rowNorms []float64
}

func newKLDivergenceWorkspace(nSamples int, nComponents int) *klDivergenceWorkspace {
	return &klDivergenceWorkspace{
		num:      make([]float64, nSamples*nSamples),
		q:        make([]float64, nSamples*nSamples),
		grad:     make([]float64, nSamples*nComponents),
		rowNorms: make([]float64, nSamples),
	}
}

func (w *klDivergenceWorkspace) reset() {
	clear(w.grad)
}

type gradientDescentOptions struct {
	startIter             int
	maxIter               int
	nIterCheck            int
	nIterWithoutProgress  int
	momentum              float64
	learningRate          float64
	minGain               float64
	minGradNorm           float64
	computeObjectiveValue bool
}

func gradientDescent(
	initial []float64,
	opts gradientDescentOptions,
	objective func(params []float64, computeError bool) (float64, []float64),
) ([]float64, float64, int) {
	params := slices.Clone(initial)
	update := make([]float64, len(params))
	gains := make([]float64, len(params))
	for i := range gains {
		gains[i] = 1.0
	}

	bestError := math.MaxFloat64
	bestIter := opts.startIter
	currentError := math.MaxFloat64
	lastIter := opts.startIter

	for iter := opts.startIter; iter < opts.maxIter; iter++ {
		lastIter = iter
		checkConvergence := (iter+1)%opts.nIterCheck == 0
		computeError := opts.computeObjectiveValue && (checkConvergence || iter == opts.maxIter-1)

		currentError, gradient := objective(params, computeError)

		for idx := range params {
			if update[idx]*gradient[idx] < 0 {
				gains[idx] += 0.2
			} else {
				gains[idx] *= 0.8
			}
			if gains[idx] < opts.minGain {
				gains[idx] = opts.minGain
			}
			gradient[idx] *= gains[idx]
			update[idx] = opts.momentum*update[idx] - opts.learningRate*gradient[idx]
			params[idx] += update[idx]
		}

		if !checkConvergence {
			continue
		}

		gradNorm := floats.Norm(gradient, 2)
		if currentError < bestError {
			bestError = currentError
			bestIter = iter
		} else if iter-bestIter > opts.nIterWithoutProgress {
			break
		}
		if gradNorm <= opts.minGradNorm {
			break
		}
	}

	return params, currentError, lastIter
}

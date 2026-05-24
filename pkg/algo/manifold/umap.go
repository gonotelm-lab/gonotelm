package manifold

import (
	"fmt"
	"slices"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
	umaplib "github.com/nozzle/umap"
)

const (
	defaultUMAPNNeighbors               = 15
	defaultUMAPMinDist                  = 0.1
	defaultUMAPSpread                   = 1.0
	defaultUMAPNEpochs                  = 0 // 0 means library auto-select.
	defaultUMAPLearningRate             = 1.0
	defaultUMAPNegativeSampleRate       = 5
	defaultUMAPMetric                   = "euclidean"
	defaultUMAPLocalConnectivity        = 1.0
	defaultUMAPSetOpMixRatio            = 1.0
	defaultUMAPSeed               int64 = 42
)

// UMAPInitMethod controls low-dimensional initialization mode.
type UMAPInitMethod string

const (
	// UMAPInitSpectral initializes with spectral embedding.
	UMAPInitSpectral UMAPInitMethod = "spectral"
	// UMAPInitRandom initializes with random coordinates.
	UMAPInitRandom UMAPInitMethod = "random"
)

// UMAPOption configures a UMAP model.
type UMAPOption func(*UMAP)

// WithUMAPNNeighbors sets neighborhood size used to build the graph (default: 15).
//
// Range:
// - Hard constraint: [2, n_samples-1]
// - Practical tuning: [5, 100]
//
// Tuning guide:
// - Smaller (5-15): stronger local detail, more fragmented manifold.
// - Larger (30+): smoother global structure, local micro-clusters may merge.
func WithUMAPNNeighbors(value int) UMAPOption {
	return func(u *UMAP) {
		u.nNeighbors = value
	}
}

// WithUMAPMinDist sets minimum distance in low-dimensional space (default: 0.1).
//
// Range:
// - Hard constraint: [0, +inf)
// - Practical tuning: [0.0, 0.99]
//
// Tuning guide:
// - Lower values pack points tighter and sharpen clusters.
// - Higher values spread points and preserve continuity.
func WithUMAPMinDist(value float64) UMAPOption {
	return func(u *UMAP) {
		u.minDist = value
	}
}

// WithUMAPSpread sets effective embedding scale (default: 1.0).
//
// Range:
// - Hard constraint: (0, +inf)
// - Practical tuning: [0.5, 3.0]
//
// Often tuned together with min_dist; larger spread opens inter-cluster spacing.
func WithUMAPSpread(value float64) UMAPOption {
	return func(u *UMAP) {
		u.spread = value
	}
}

// WithUMAPNEpochs sets optimization epochs.
// Value <= 0 delegates epoch count selection to the underlying library.
//
// Range:
// - Hard constraint: integer (<=0 means auto, >0 means explicit epochs)
// - Practical tuning: [100, 2000] when explicitly set
//
// Increase when embedding is still noisy or not converged.
func WithUMAPNEpochs(value int) UMAPOption {
	return func(u *UMAP) {
		u.nEpochs = value
	}
}

// WithUMAPLearningRate sets initial SGD learning rate (default: 1.0).
//
// Range:
// - Hard constraint: (0, +inf)
// - Practical tuning: [0.01, 10.0]
//
// Too high may destabilize layout; too low slows convergence.
func WithUMAPLearningRate(value float64) UMAPOption {
	return func(u *UMAP) {
		u.learningRate = value
	}
}

// WithUMAPNegativeSampleRate sets negative samples per positive edge (default: 5).
//
// Range:
// - Hard constraint: integer >= 1
// - Practical tuning: [2, 20]
//
// Larger values strengthen repulsion, improving cluster separation at higher cost.
func WithUMAPNegativeSampleRate(value int) UMAPOption {
	return func(u *UMAP) {
		u.negativeSampleRate = value
	}
}

// WithUMAPMetric sets distance metric for graph construction (default: "euclidean").
//
// Range:
//   - Hard constraint: one of UMAPMetrics()
//   - Supported names: euclidean, manhattan, chebyshev, minkowski, canberra, braycurtis,
//     cosine, correlation, hamming, jaccard, dice
//
// Choose metric to match feature semantics, e.g. "cosine" for normalized embeddings.
func WithUMAPMetric(metric string) UMAPOption {
	return func(u *UMAP) {
		u.metric = metric
	}
}

// WithUMAPInit sets embedding initialization method (default: spectral).
//
// Range:
// - Hard constraint: UMAPInitSpectral or UMAPInitRandom
//
// - UMAPInitSpectral: more stable global scaffold.
// - UMAPInitRandom: useful for robustness checks across seeds.
func WithUMAPInit(method UMAPInitMethod) UMAPOption {
	return func(u *UMAP) {
		u.init = method
	}
}

// WithUMAPRandomSeed sets random seed (default: 42) for reproducibility.
//
// Range:
// - Hard constraint: any int64
// - Practical guidance: keep fixed across experiments for reproducibility.
func WithUMAPRandomSeed(seed int64) UMAPOption {
	return func(u *UMAP) {
		u.randomSeed = seed
	}
}

// WithUMAPLocalConnectivity sets local connectivity regularization (default: 1.0).
//
// Range:
// - Hard constraint: [0, +inf)
// - Practical tuning: [0.0, 5.0]
//
// Increase when manifolds have sparse bridges and local continuity is weak.
func WithUMAPLocalConnectivity(value float64) UMAPOption {
	return func(u *UMAP) {
		u.localConnectivity = value
	}
}

// WithUMAPSetOpMixRatio sets fuzzy-union vs fuzzy-intersection blend ratio (default: 1.0).
//
// Range:
// - Hard constraint: [0, 1]
// - Practical tuning: [0.0, 1.0]
//
// 1.0 is more union-like (preserve connectivity), 0.0 is more intersection-like.
func WithUMAPSetOpMixRatio(value float64) UMAPOption {
	return func(u *UMAP) {
		u.setOpMixRatio = value
	}
}

// WithUMAPNumWorkers sets worker count.
// 0 means using library default parallelism.
//
// Range:
// - Hard constraint: integer >= 0
// - Practical tuning: 0 or [1, CPU cores]
//
// Increase for faster preprocessing on CPU-rich environments.
func WithUMAPNumWorkers(value int) UMAPOption {
	return func(u *UMAP) {
		u.numWorkers = value
	}
}

// UMAP implements Uniform Manifold Approximation and Projection.
type UMAP struct {
	nComponents        int
	nNeighbors         int
	minDist            float64
	spread             float64
	nEpochs            int
	learningRate       float64
	negativeSampleRate int
	metric             string
	init               UMAPInitMethod
	randomSeed         int64
	localConnectivity  float64
	setOpMixRatio      float64
	numWorkers         int

	fitted    bool
	nFeatures int
	embedding [][]float64
}

// NewUMAP creates a UMAP model.
//
// nComponents means target embedding dimension after dimensionality reduction:
// - 2 for standard 2D visualization
// - 3 for 3D visualization
// - Hard constraint: integer >= 1
//
// Practical defaults:
// n_neighbors=15, min_dist=0.1, spread=1.0, metric="euclidean".
//
// Example:
//
//	model, err := NewUMAP(
//		2,
//		WithUMAPMetric("cosine"),
//		WithUMAPNNeighbors(30),
//		WithUMAPMinDist(0.05),
//		WithUMAPRandomSeed(42),
//	)
//	if err != nil {
//		return err
//	}
func NewUMAP(nComponents int, opts ...UMAPOption) (*UMAP, error) {
	model := &UMAP{
		nComponents:        nComponents,
		nNeighbors:         defaultUMAPNNeighbors,
		minDist:            defaultUMAPMinDist,
		spread:             defaultUMAPSpread,
		nEpochs:            defaultUMAPNEpochs,
		learningRate:       defaultUMAPLearningRate,
		negativeSampleRate: defaultUMAPNegativeSampleRate,
		metric:             defaultUMAPMetric,
		init:               UMAPInitSpectral,
		randomSeed:         defaultUMAPSeed,
		localConnectivity:  defaultUMAPLocalConnectivity,
		setOpMixRatio:      defaultUMAPSetOpMixRatio,
		numWorkers:         0,
	}
	for _, opt := range opts {
		opt(model)
	}
	if err := model.validateStaticConfig(); err != nil {
		return nil, fmt.Errorf("invalid UMAP config: %w", err)
	}
	return model, nil
}

// Fit trains UMAP and stores embedding.
func (u *UMAP) Fit(data [][]float64) error {
	embedding, err := u.FitTransform(data)
	if err != nil {
		return err
	}
	u.embedding = embedding
	return nil
}

// FitTransform trains UMAP and returns the low-dimensional embedding.
func (u *UMAP) FitTransform(data [][]float64) (result [][]float64, err error) {
	nSamples, nFeatures, err := numutil.Validate2DFloat64(data, 2)
	if err != nil {
		return nil, err
	}
	if err := u.validateDataDependentConfig(nSamples); err != nil {
		return nil, err
	}

	// Protect callers from third-party panic paths.
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("umap fit failed: %v", recovered)
		}
	}()

	float32Data := numutil.Float64ToFloat32Matrix(data)
	config := umaplib.DefaultConfig()
	config.NNeighbors = u.nNeighbors
	config.NComponents = u.nComponents
	config.Metric = u.metric
	config.MinDist = float32(u.minDist)
	config.Spread = float32(u.spread)
	config.NEpochs = u.nEpochs
	config.LearningRate = float32(u.learningRate)
	config.NegativeSampleRate = u.negativeSampleRate
	config.Init = string(u.init)
	config.LocalConnectivity = u.localConnectivity
	config.SetOpMixRatio = u.setOpMixRatio
	config.Seed = u.randomSeed
	config.NumWorkers = u.numWorkers
	config.Verbose = false

	model := umaplib.New(config)
	embedding32 := model.FitTransform(float32Data)
	embedding64 := numutil.Float32ToFloat64Matrix(embedding32)
	if err := validateEmbedding(embedding64, nSamples, u.nComponents); err != nil {
		return nil, err
	}

	u.fitted = true
	u.nFeatures = nFeatures
	u.embedding = embedding64
	return numutil.Clone2DFloat64(embedding64), nil
}

// Embedding returns a copy of fitted embedding.
func (u *UMAP) Embedding() [][]float64 {
	return numutil.Clone2DFloat64(u.embedding)
}

func (u *UMAP) validateStaticConfig() error {
	if u.nComponents <= 0 {
		return fmt.Errorf("n_components must be positive, got %d", u.nComponents)
	}
	if u.nNeighbors < 2 {
		return fmt.Errorf("n_neighbors must be >= 2, got %d", u.nNeighbors)
	}
	if u.minDist < 0 {
		return fmt.Errorf("min_dist must be >= 0, got %.6f", u.minDist)
	}
	if u.spread <= 0 {
		return fmt.Errorf("spread must be > 0, got %.6f", u.spread)
	}
	if u.learningRate <= 0 {
		return fmt.Errorf("learning_rate must be > 0, got %.6f", u.learningRate)
	}
	if u.negativeSampleRate <= 0 {
		return fmt.Errorf("negative_sample_rate must be > 0, got %d", u.negativeSampleRate)
	}
	if u.localConnectivity < 0 {
		return fmt.Errorf("local_connectivity must be >= 0, got %.6f", u.localConnectivity)
	}
	if u.setOpMixRatio < 0 || u.setOpMixRatio > 1 {
		return fmt.Errorf("set_op_mix_ratio must be within [0, 1], got %.6f", u.setOpMixRatio)
	}
	if u.numWorkers < 0 {
		return fmt.Errorf("num_workers must be >= 0, got %d", u.numWorkers)
	}
	if u.init != UMAPInitSpectral && u.init != UMAPInitRandom {
		return fmt.Errorf("unsupported init method: %q", u.init)
	}
	if !isSupportedUMAPMetric(u.metric) {
		return fmt.Errorf("unsupported metric: %q", u.metric)
	}
	return nil
}

func (u *UMAP) validateDataDependentConfig(nSamples int) error {
	if u.nNeighbors >= nSamples {
		return fmt.Errorf("n_neighbors must be < n_samples, got n_neighbors=%d n_samples=%d",
			u.nNeighbors, nSamples)
	}
	return nil
}

func isSupportedUMAPMetric(metric string) bool {
	_, ok := supportedUMAPMetrics[metric]
	return ok
}

var supportedUMAPMetrics = map[string]struct{}{
	"euclidean":   {},
	"manhattan":   {},
	"chebyshev":   {},
	"minkowski":   {},
	"canberra":    {},
	"braycurtis":  {},
	"cosine":      {},
	"correlation": {},
	"hamming":     {},
	"jaccard":     {},
	"dice":        {},
}

var supportedUMAPMetricNames = func() []string {
	keys := make([]string, 0, len(supportedUMAPMetrics))
	for key := range supportedUMAPMetrics {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}()

func validateEmbedding(embedding [][]float64, expectedRows int, expectedCols int) error {
	if len(embedding) != expectedRows {
		return fmt.Errorf("embedding row mismatch: got=%d want=%d", len(embedding), expectedRows)
	}
	if expectedRows == 0 {
		return nil
	}
	if err := numutil.ValidateRowsWithFeatureCount(embedding, expectedCols); err != nil {
		return fmt.Errorf("embedding validation failed: %w", err)
	}
	return nil
}

// Metrics returns supported metric names in sorted order.
func UMAPMetrics() []string {
	return slices.Clone(supportedUMAPMetricNames)
}

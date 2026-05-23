package algo

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"slices"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distmv"
)

// GMMOptions controls training behavior of GMMCluster.
type GMMOptions struct {
	// MaxIterations is the maximum number of EM iterations.
	// Default: 100.
	MaxIterations int

	// Tolerance is the absolute convergence threshold of log-likelihood.
	// Default: 1e-6.
	Tolerance float64

	// Regularization is added to covariance diagonal to avoid singular matrices.
	// Default: 1e-6.
	Regularization float64

	// Seed is used by centroid initialization.
	// Default: 1.
	Seed int64

	// NumInitializations is the number of EM restarts, choose the best log-likelihood.
	// Default: 2.
	NumInitializations int

	// KMeansMaxIterations is the max iterations for kmeans initialization.
	// Default: 20.
	KMeansMaxIterations int

	// AutoMaxClusters is the upper bound of K when clusterCount = -1.
	// Default: 50.
	AutoMaxClusters int
}

// GMMResult contains the clustering outcome.
type GMMResult struct {
	Labels           []int
	Responsibilities [][]float64
	Weights          []float64
	Means            [][]float64
	ClusterCount     int
	LogLikelihood    float64
	BIC              float64
	Iterations       int
}

// GMMCluster clusters vectors with Gaussian Mixture Model (EM algorithm).
//
// Input vectors must have consistent dimensions.
// clusterCount is the number of Gaussian components.
// clusterCount < 0 means auto-select K by minimum BIC in [2, min(sampleCount, AutoMaxClusters)].
func GMMCluster(vectors [][]float64, clusterCount int, opts *GMMOptions) (*GMMResult, error) {
	samples, dim, err := validateVectors(vectors)
	if err != nil {
		return nil, err
	}

	cfg, err := normalizeGMMOptions(opts)
	if err != nil {
		return nil, err
	}

	// 全局协方差只依赖样本和 regularization，缓存一次供所有 K/重启复用。
	globalCov := initCovariances(samples, 1, cfg.Regularization)[0]

	if clusterCount < 0 {
		return gmmClusterAutoSelectK(samples, dim, cfg, globalCov)
	}
	if clusterCount == 0 {
		return nil, fmt.Errorf("cluster count must not be 0")
	}
	if clusterCount > len(samples) {
		return nil, fmt.Errorf("cluster count must not exceed sample count")
	}

	return gmmClusterFixedK(samples, dim, clusterCount, cfg, globalCov)
}

// GMMClusterAutoK clusters vectors with automatic K selection by BIC.
func GMMClusterAutoK(vectors [][]float64, opts *GMMOptions) (*GMMResult, error) {
	return GMMCluster(vectors, -1, opts)
}

func gmmClusterAutoSelectK(
	samples [][]float64,
	dim int,
	cfg *normalizedGMMOptions,
	globalCov *mat.SymDense,
) (*GMMResult, error) {
	if len(samples) < 2 {
		return nil, fmt.Errorf("auto-select cluster count requires at least 2 samples")
	}
	maxClusterCount := min(len(samples), cfg.AutoMaxClusters)
	if maxClusterCount < 2 {
		return nil, fmt.Errorf("auto max clusters must be at least 2 in auto mode")
	}

	type autoKCandidate struct {
		k      int
		result *GMMResult
		err    error
	}

	candidateCount := maxClusterCount - 1
	maxParallel := min(runtime.GOMAXPROCS(0), candidateCount)
	if maxParallel <= 0 {
		maxParallel = 1
	}
	sem := make(chan struct{}, maxParallel)
	outcomes := make(chan autoKCandidate, candidateCount)
	var wg sync.WaitGroup

	for k := 2; k <= maxClusterCount; k++ {
		k := k
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			result, err := gmmClusterFixedK(samples, dim, k, cfg, globalCov)
			<-sem
			outcomes <- autoKCandidate{k: k, result: result, err: err}
		}()
	}
	go func() {
		wg.Wait()
		close(outcomes)
	}()

	bestBIC := math.Inf(1)
	bestK := 0
	var bestResult *GMMResult
	var firstErr error
	firstErrK := 0

	for candidate := range outcomes {
		if candidate.err != nil {
			if firstErr == nil || candidate.k < firstErrK {
				firstErr = candidate.err
				firstErrK = candidate.k
			}
			continue
		}
		if bestResult == nil ||
			candidate.result.BIC < bestBIC ||
			(math.Abs(candidate.result.BIC-bestBIC) <= 1e-12 && candidate.k < bestK) {
			bestResult = candidate.result
			bestBIC = candidate.result.BIC
			bestK = candidate.k
		}
	}

	if firstErr != nil {
		return nil, fmt.Errorf("auto-select cluster count failed at k=%d: %w", firstErrK, firstErr)
	}
	if bestResult == nil {
		return nil, fmt.Errorf("auto-select cluster count produced no candidate")
	}
	return bestResult, nil
}

func gmmClusterFixedK(
	samples [][]float64,
	dim int,
	clusterCount int,
	cfg *normalizedGMMOptions,
	globalCov *mat.SymDense,
) (*GMMResult, error) {
	if globalCov == nil {
		globalCov = initCovariances(samples, 1, cfg.Regularization)[0]
	}

	restartCount := max(1, cfg.NumInitializations)
	bestLogLikelihood := math.Inf(-1)
	var (
		bestState *gmmEMState
		lastErr   error
	)

	for restart := 0; restart < restartCount; restart++ {
		initSeed := cfg.Seed + int64(restart)*7919
		means, covariances, weights, err := initializeModelFromKMeans(
			samples,
			dim,
			clusterCount,
			initSeed,
			cfg.KMeansMaxIterations,
			cfg.Regularization,
			globalCov,
		)
		if err != nil {
			lastErr = err
			continue
		}

		state, err := runGMMEM(
			samples,
			dim,
			clusterCount,
			means,
			covariances,
			weights,
			globalCov,
			cfg,
		)
		if err != nil {
			lastErr = err
			continue
		}

		if bestState == nil || state.LogLikelihood > bestLogLikelihood {
			bestState = state
			bestLogLikelihood = state.LogLikelihood
		}
	}

	if bestState == nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("gmm failed for all initializations")
	}

	bic := calculateGMMBIC(bestState.LogLikelihood, len(samples), dim, clusterCount)

	return &GMMResult{
		Labels:           append([]int(nil), bestState.Labels...),
		Responsibilities: copy2DSlice(bestState.Responsibilities),
		Weights:          append([]float64(nil), bestState.Weights...),
		Means:            copy2DSlice(bestState.Means),
		ClusterCount:     clusterCount,
		LogLikelihood:    bestState.LogLikelihood,
		BIC:              bic,
		Iterations:       bestState.Iterations,
	}, nil
}

type gmmEMState struct {
	Labels           []int
	Responsibilities [][]float64
	Weights          []float64
	Means            [][]float64
	LogLikelihood    float64
	Iterations       int
}

func runGMMEM(
	samples [][]float64,
	dim int,
	clusterCount int,
	means [][]float64,
	covariances []*mat.SymDense,
	weights []float64,
	globalCov *mat.SymDense,
	cfg *normalizedGMMOptions,
) (*gmmEMState, error) {
	resp := make([][]float64, len(samples))
	for i := range resp {
		resp[i] = make([]float64, clusterCount)
	}

	prevLowerBound := math.Inf(-1)
	iterations := 0

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		iterations = iter + 1

		components, err := buildGaussians(means, covariances, cfg.Regularization)
		if err != nil {
			return nil, err
		}

		logLikelihood := expectationStep(samples, components, weights, cfg.Regularization, resp)
		means, covariances, weights = maximizationStep(
			samples,
			resp,
			dim,
			clusterCount,
			cfg.Regularization,
			globalCov,
		)

		lowerBound := logLikelihood / float64(len(samples))
		if iter > 0 && math.Abs(lowerBound-prevLowerBound) <= cfg.Tolerance {
			break
		}
		prevLowerBound = lowerBound
	}

	// Use final parameters to produce final responsibilities and labels.
	components, err := buildGaussians(means, covariances, cfg.Regularization)
	if err != nil {
		return nil, err
	}
	logLikelihood := expectationStep(samples, components, weights, cfg.Regularization, resp)
	labels := labelsFromResponsibilities(resp)

	return &gmmEMState{
		Labels:           labels,
		Responsibilities: copy2DSlice(resp),
		Weights:          append([]float64(nil), weights...),
		Means:            copy2DSlice(means),
		LogLikelihood:    logLikelihood,
		Iterations:       iterations,
	}, nil
}

func expectationStep(
	samples [][]float64,
	components []*distmv.Normal,
	weights []float64,
	regularization float64,
	resp [][]float64,
) float64 {
	logLikelihood := 0.0
	logProb := make([]float64, len(components))

	for i := range samples {
		for c := range components {
			weight := math.Max(weights[c], regularization)
			logProb[c] = math.Log(weight) + components[c].LogProb(samples[i])
		}
		lse := floats.LogSumExp(logProb)
		logLikelihood += lse
		for c := range components {
			resp[i][c] = math.Exp(logProb[c] - lse)
		}
	}
	return logLikelihood
}

func maximizationStep(
	samples [][]float64,
	resp [][]float64,
	dim int,
	clusterCount int,
	regularization float64,
	globalCov *mat.SymDense,
) ([][]float64, []*mat.SymDense, []float64) {
	nk := make([]float64, clusterCount)
	for i := range samples {
		for c := 0; c < clusterCount; c++ {
			nk[c] += resp[i][c]
		}
	}

	means := make([][]float64, clusterCount)
	covariances := make([]*mat.SymDense, clusterCount)
	weights := make([]float64, clusterCount)

	for c := 0; c < clusterCount; c++ {
		if nk[c] < regularization {
			means[c] = append([]float64(nil), samples[c%len(samples)]...)
			covariances[c] = cloneSymDense(globalCov)
			weights[c] = regularization
			continue
		}

		mean := make([]float64, dim)
		invNk := 1.0 / nk[c]
		for i := range samples {
			r := resp[i][c]
			for d := 0; d < dim; d++ {
				mean[d] += r * samples[i][d]
			}
		}
		for d := 0; d < dim; d++ {
			mean[d] *= invNk
		}
		means[c] = mean

		cov := mat.NewSymDense(dim, nil)
		for i := range samples {
			r := resp[i][c]
			if r == 0 {
				continue
			}
			for a := 0; a < dim; a++ {
				da := samples[i][a] - mean[a]
				for b := 0; b <= a; b++ {
					db := samples[i][b] - mean[b]
					cov.SetSym(a, b, cov.At(a, b)+r*da*db)
				}
			}
		}
		for a := 0; a < dim; a++ {
			for b := 0; b <= a; b++ {
				cov.SetSym(a, b, cov.At(a, b)*invNk)
			}
		}
		for d := 0; d < dim; d++ {
			cov.SetSym(d, d, cov.At(d, d)+regularization)
		}
		covariances[c] = cov

		weights[c] = nk[c] / float64(len(samples))
	}
	normalizeWeights(weights)
	return means, covariances, weights
}

func initializeModelFromKMeans(
	samples [][]float64,
	dim int,
	clusterCount int,
	seed int64,
	kmeansMaxIterations int,
	regularization float64,
	globalCov *mat.SymDense,
) ([][]float64, []*mat.SymDense, []float64, error) {
	means, labels, counts := runKMeans(samples, clusterCount, seed, kmeansMaxIterations)
	if len(means) != clusterCount {
		return nil, nil, nil, fmt.Errorf("kmeans init returned invalid center count")
	}
	covariances := make([]*mat.SymDense, clusterCount)
	weights := make([]float64, clusterCount)

	for c := 0; c < clusterCount; c++ {
		if counts[c] <= 1 {
			covariances[c] = cloneSymDense(globalCov)
			weights[c] = regularization
			continue
		}
		cov := empiricalClusterCovariance(samples, labels, c, means[c], dim, regularization)
		covariances[c] = cov
		weights[c] = float64(counts[c]) / float64(len(samples))
	}
	normalizeWeights(weights)

	return means, covariances, weights, nil
}

func runKMeans(
	samples [][]float64,
	clusterCount int,
	seed int64,
	maxIterations int,
) ([][]float64, []int, []int) {
	rng := rand.New(rand.NewSource(seed))
	means := initKMeansPlusPlusCenters(samples, clusterCount, rng)

	labels := make([]int, len(samples))
	for i := range labels {
		labels[i] = -1
	}
	counts := make([]int, clusterCount)

	for iter := 0; iter < max(1, maxIterations); iter++ {
		changed := false
		for i := range counts {
			counts[i] = 0
		}
		sums := make([][]float64, clusterCount)
		for c := 0; c < clusterCount; c++ {
			sums[c] = make([]float64, len(samples[0]))
		}

		for i := range samples {
			label := findNearestCenter(samples[i], means)
			if labels[i] != label {
				changed = true
				labels[i] = label
			}
			counts[label]++
			for d := 0; d < len(samples[i]); d++ {
				sums[label][d] += samples[i][d]
			}
		}

		for c := 0; c < clusterCount; c++ {
			if counts[c] == 0 {
				means[c] = append([]float64(nil), samples[rng.Intn(len(samples))]...)
				continue
			}
			inv := 1.0 / float64(counts[c])
			for d := 0; d < len(means[c]); d++ {
				means[c][d] = sums[c][d] * inv
			}
		}

		if !changed && iter > 0 {
			break
		}
	}

	// Recount labels for the final means.
	for i := range counts {
		counts[i] = 0
	}
	for i := range samples {
		labels[i] = findNearestCenter(samples[i], means)
		counts[labels[i]]++
	}

	return means, labels, counts
}

func initKMeansPlusPlusCenters(samples [][]float64, clusterCount int, rng *rand.Rand) [][]float64 {
	centers := make([][]float64, 0, clusterCount)
	firstIdx := rng.Intn(len(samples))
	centers = append(centers, append([]float64(nil), samples[firstIdx]...))

	distances := make([]float64, len(samples))
	for len(centers) < clusterCount {
		total := 0.0
		for i := range samples {
			minDist := squaredDistance(samples[i], centers[0])
			for c := 1; c < len(centers); c++ {
				d := squaredDistance(samples[i], centers[c])
				if d < minDist {
					minDist = d
				}
			}
			distances[i] = minDist
			total += minDist
		}

		if total == 0 {
			centers = append(centers, append([]float64(nil), samples[rng.Intn(len(samples))]...))
			continue
		}

		target := rng.Float64() * total
		cumulative := 0.0
		chosen := len(samples) - 1
		for i, d := range distances {
			cumulative += d
			if cumulative >= target {
				chosen = i
				break
			}
		}
		centers = append(centers, append([]float64(nil), samples[chosen]...))
	}
	return centers
}

func findNearestCenter(sample []float64, means [][]float64) int {
	best := 0
	bestDist := squaredDistance(sample, means[0])
	for c := 1; c < len(means); c++ {
		d := squaredDistance(sample, means[c])
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func empiricalClusterCovariance(
	samples [][]float64,
	labels []int,
	cluster int,
	mean []float64,
	dim int,
	regularization float64,
) *mat.SymDense {
	cov := mat.NewSymDense(dim, nil)
	count := 0

	for i := range samples {
		if labels[i] != cluster {
			continue
		}
		count++
		for a := 0; a < dim; a++ {
			da := samples[i][a] - mean[a]
			for b := 0; b <= a; b++ {
				db := samples[i][b] - mean[b]
				cov.SetSym(a, b, cov.At(a, b)+da*db)
			}
		}
	}

	denominator := math.Max(1, float64(count))
	for a := 0; a < dim; a++ {
		for b := 0; b <= a; b++ {
			cov.SetSym(a, b, cov.At(a, b)/denominator)
		}
		cov.SetSym(a, a, cov.At(a, a)+regularization)
	}
	return cov
}

func normalizeWeights(weights []float64) {
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if sum <= 0 {
		for i := range weights {
			weights[i] = 1.0 / float64(len(weights))
		}
		return
	}
	inv := 1.0 / sum
	for i := range weights {
		weights[i] *= inv
	}
}

func labelsFromResponsibilities(resp [][]float64) []int {
	labels := make([]int, len(resp))
	for i := range resp {
		best := 0
		bestValue := resp[i][0]
		for c := 1; c < len(resp[i]); c++ {
			if resp[i][c] > bestValue {
				best = c
				bestValue = resp[i][c]
			}
		}
		labels[i] = best
	}
	return labels
}

func calculateGMMBIC(logLikelihood float64, sampleCount, dim, clusterCount int) float64 {
	// Full-covariance GMM parameter count:
	// mixing weights (k-1) + means (k*d) + covariances (k*d*(d+1)/2).
	paramCount := float64((clusterCount - 1) + clusterCount*dim + clusterCount*dim*(dim+1)/2)
	return -2*logLikelihood + paramCount*math.Log(float64(sampleCount))
}

type normalizedGMMOptions struct {
	MaxIterations       int
	Tolerance           float64
	Regularization      float64
	Seed                int64
	NumInitializations  int
	KMeansMaxIterations int
	AutoMaxClusters     int
}

func normalizeGMMOptions(opts *GMMOptions) (*normalizedGMMOptions, error) {
	cfg := &normalizedGMMOptions{
		MaxIterations:       100,
		Tolerance:           1e-6,
		Regularization:      1e-6,
		Seed:                1,
		NumInitializations:  2,
		KMeansMaxIterations: 20,
		AutoMaxClusters:     50,
	}
	if opts == nil {
		return cfg, nil
	}

	if opts.MaxIterations < 0 {
		return nil, fmt.Errorf("max iterations must not be negative")
	}
	if opts.Tolerance < 0 {
		return nil, fmt.Errorf("tolerance must not be negative")
	}
	if opts.Regularization < 0 {
		return nil, fmt.Errorf("regularization must not be negative")
	}
	if opts.NumInitializations < 0 {
		return nil, fmt.Errorf("num initializations must not be negative")
	}
	if opts.KMeansMaxIterations < 0 {
		return nil, fmt.Errorf("kmeans max iterations must not be negative")
	}
	if opts.AutoMaxClusters < 0 {
		return nil, fmt.Errorf("auto max clusters must not be negative")
	}

	if opts.MaxIterations > 0 {
		cfg.MaxIterations = opts.MaxIterations
	}
	if opts.Tolerance > 0 {
		cfg.Tolerance = opts.Tolerance
	}
	if opts.Regularization > 0 {
		cfg.Regularization = opts.Regularization
	}
	if opts.Seed != 0 {
		cfg.Seed = opts.Seed
	}
	if opts.NumInitializations > 0 {
		cfg.NumInitializations = opts.NumInitializations
	}
	if opts.KMeansMaxIterations > 0 {
		cfg.KMeansMaxIterations = opts.KMeansMaxIterations
	}
	if opts.AutoMaxClusters > 0 {
		cfg.AutoMaxClusters = opts.AutoMaxClusters
	}

	return cfg, nil
}

func validateVectors(vectors [][]float64) ([][]float64, int, error) {
	if len(vectors) == 0 {
		return nil, 0, fmt.Errorf("input vectors are empty")
	}
	if len(vectors[0]) == 0 {
		return nil, 0, fmt.Errorf("vector dimension must be greater than 0")
	}

	dim := len(vectors[0])
	out := make([][]float64, len(vectors))
	for i := range vectors {
		if len(vectors[i]) != dim {
			return nil, 0, fmt.Errorf("all vectors must have the same dimension")
		}
		row := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v := vectors[i][d]
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, 0, fmt.Errorf("vector contains invalid value at row=%d col=%d", i, d)
			}
			row[d] = v
		}
		out[i] = row
	}
	return out, dim, nil
}

func initFarthestMeans(samples [][]float64, clusterCount int, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	means := make([][]float64, 0, clusterCount)
	selected := make(map[int]struct{}, clusterCount)

	first := rng.Intn(len(samples))
	selected[first] = struct{}{}
	means = append(means, append([]float64(nil), samples[first]...))

	for len(means) < clusterCount {
		bestIdx := -1
		bestDist := -1.0

		for i := range samples {
			if _, ok := selected[i]; ok {
				continue
			}
			minDist := math.Inf(1)
			for _, mean := range means {
				d := squaredDistance(samples[i], mean)
				if d < minDist {
					minDist = d
				}
			}
			if minDist > bestDist {
				bestDist = minDist
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			bestIdx = rng.Intn(len(samples))
		}
		selected[bestIdx] = struct{}{}
		means = append(means, append([]float64(nil), samples[bestIdx]...))
	}

	return means
}
func initCovariances(samples [][]float64, clusterCount int, regularization float64) []*mat.SymDense {
	dim := len(samples[0])
	mean := make([]float64, dim)
	for i := range samples {
		for d := 0; d < dim; d++ {
			mean[d] += samples[i][d]
		}
	}
	for d := 0; d < dim; d++ {
		mean[d] /= float64(len(samples))
	}

	base := mat.NewSymDense(dim, nil)
	for i := range samples {
		for a := 0; a < dim; a++ {
			da := samples[i][a] - mean[a]
			for b := 0; b <= a; b++ {
				db := samples[i][b] - mean[b]
				base.SetSym(a, b, base.At(a, b)+da*db)
			}
		}
	}
	denominator := math.Max(1, float64(len(samples)-1))
	for a := 0; a < dim; a++ {
		for b := 0; b <= a; b++ {
			base.SetSym(a, b, base.At(a, b)/denominator)
		}
		base.SetSym(a, a, base.At(a, a)+regularization)
	}

	covs := make([]*mat.SymDense, clusterCount)
	for i := 0; i < clusterCount; i++ {
		covs[i] = cloneSymDense(base)
	}
	return covs
}

func buildGaussians(means [][]float64, covariances []*mat.SymDense, regularization float64) ([]*distmv.Normal, error) {
	components := make([]*distmv.Normal, len(means))
	for i := range means {
		var comp *distmv.Normal
		for attempt := 0; attempt < 6; attempt++ {
			cov := cloneSymDense(covariances[i])
			boost := regularization * math.Pow(10, float64(attempt))
			for d := 0; d < cov.SymmetricDim(); d++ {
				cov.SetSym(d, d, cov.At(d, d)+boost)
			}
			var ok bool
			comp, ok = distmv.NewNormal(means[i], cov, nil)
			if ok {
				break
			}
		}
		if comp == nil {
			return nil, fmt.Errorf("build gaussian component %d failed", i)
		}
		components[i] = comp
	}
	return components, nil
}

func cloneSymDense(in *mat.SymDense) *mat.SymDense {
	n := in.SymmetricDim()
	out := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			out.SetSym(i, j, in.At(i, j))
		}
	}
	return out
}

func squaredDistance(a, b []float64) float64 {
	dist := floats.Distance(a, b, 2)
	return dist * dist
}

func copy2DSlice(in [][]float64) [][]float64 {
	out := make([][]float64, len(in))
	for i := range in {
		out[i] = slices.Clone(in[i])
	}
	return out
}

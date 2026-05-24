package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/decomposition"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

const (
	drGMMUVPythonPath = "../.venv/bin/python"
	drGMMFeatureLimit = 24
	drGMMSeed         = 42
)

type drGMMMethodResult struct {
	Name                string      `json:"name"`
	Embedding           [][]float64 `json:"embedding"`
	Labels              []int       `json:"labels"`
	Means               [][]float64 `json:"means"`
	Trustworthiness     float64     `json:"trustworthiness"`
	DistanceCorrelation float64     `json:"distance_correlation"`
	LogLikelihood       float64     `json:"log_likelihood"`
	BIC                 float64     `json:"bic"`
}

type drGMMPythonSnapshotRequest struct {
	Vectors        [][]float64 `json:"vectors"`
	TSNEPerplexity float64     `json:"tsne_perplexity"`
	TrustNeighbors int         `json:"trust_neighbors"`
	ClusterCount   int         `json:"cluster_count"`
	Seed           int         `json:"seed"`
}

type drGMMPythonSnapshotResponse struct {
	Methods []drGMMMethodResult `json:"methods"`
}

type drGMMPythonSnapshotResult struct {
	methods []drGMMMethodResult
	stderr  string
	err     error
}

type drGMMGoVsPythonPlotPayload struct {
	DatasetName   string              `json:"dataset_name"`
	GoMethods     []drGMMMethodResult `json:"go_methods"`
	PythonMethods []drGMMMethodResult `json:"python_methods"`
	OutputFile    string              `json:"output_file,omitempty"`
}

type drGMMStitchDataset struct {
	Name      string `json:"name"`
	ImageFile string `json:"image_file"`
}

type drGMMStitchPayload struct {
	Title      string               `json:"title,omitempty"`
	Datasets   []drGMMStitchDataset `json:"datasets"`
	OutputFile string               `json:"output_file,omitempty"`
}

func TestIntegration_DRPlusGMM_CompareGoAndPython(t *testing.T) {
	cases := []drDatasetCase{
		{Name: "embeddings_1", File: "embeddings_1.jsonl"},
		{Name: "embeddings_2", File: "embeddings_2.jsonl"},
		{Name: "embeddings_3", File: "embeddings_3.jsonl"},
	}

	allPassed := true
	for _, tc := range cases {
		tc := tc
		ok := t.Run(tc.Name, func(t *testing.T) {
			vectors := loadJSONLVectors(t, filepath.Join("testdata", tc.File))
			vectors = truncateFeatureDim(vectors, drGMMFeatureLimit)
			tsnePerplexity := resolvePerplexity(len(vectors))
			clusterCount := resolveDRGMMClusterCount(len(vectors))

			pythonResultCh := make(chan drGMMPythonSnapshotResult, 1)
			go func() {
				methods, stderr, err := loadPythonDRGMMSnapshotViaUV(
					vectors,
					tsnePerplexity,
					drTrustNeighbors,
					clusterCount,
					drGMMSeed,
				)
				pythonResultCh <- drGMMPythonSnapshotResult{
					methods: methods,
					stderr:  stderr,
					err:     err,
				}
			}()

			highCondensed := numutil.CondensedPairwiseDistances(vectors)
			highNormalized := numutil.NormalizeByMean(highCondensed)

			pca := decomposition.NewPCA(2)
			pcaEmbedding, err := pca.FitTransform(vectors)
			if err != nil {
				t.Fatalf("pca fit_transform failed: %v", err)
			}
			tsne, err := manifold.NewTSNE(
				2,
				manifold.WithPerplexity(tsnePerplexity),
				manifold.WithEarlyExaggeration(12),
				manifold.WithAutoLearningRate(),
				manifold.WithMaxIter(350),
				manifold.WithNIterWithoutProgress(200),
				manifold.WithMinGradNorm(1e-7),
				manifold.WithInitMethod(manifold.InitPCA),
				manifold.WithRandomSeed(drGMMSeed),
			)
			if err != nil {
				t.Fatalf("new tsne failed: %v", err)
			}
			tsneEmbedding, err := tsne.FitTransform(vectors)
			if err != nil {
				t.Fatalf("tsne fit_transform failed: %v", err)
			}
			umapModel, err := manifold.NewUMAP(
				2,
				manifold.WithUMAPNNeighbors(15),
				manifold.WithUMAPMinDist(0.1),
				manifold.WithUMAPSpread(1.0),
				manifold.WithUMAPMetric("euclidean"),
				manifold.WithUMAPInit(manifold.UMAPInitSpectral),
				manifold.WithUMAPRandomSeed(drGMMSeed),
			)
			if err != nil {
				t.Fatalf("new umap failed: %v", err)
			}
			umapEmbedding, err := umapModel.FitTransform(vectors)
			if err != nil {
				t.Fatalf("umap fit_transform failed: %v", err)
			}

			goResults := make([]drGMMMethodResult, 0, 3)
			for _, method := range []struct {
				name      string
				embedding [][]float64
			}{
				{name: "PCA", embedding: pcaEmbedding},
				{name: "t-SNE", embedding: tsneEmbedding},
				{name: "UMAP", embedding: umapEmbedding},
			} {
				result, err := buildGoDRGMMMethodResult(
					method.name,
					vectors,
					method.embedding,
					highNormalized,
					drTrustNeighbors,
					clusterCount,
					drGMMSeed,
				)
				if err != nil {
					t.Fatalf("build go method result failed for %s: %v", method.name, err)
				}
				goResults = append(goResults, result)
			}

			pythonResult := <-pythonResultCh
			if pythonResult.err != nil {
				t.Fatalf(
					"run python dr+gmm snapshot via uv failed: %v\nstderr:\n%s",
					pythonResult.err,
					pythonResult.stderr,
				)
			}

			goByName := make(map[string]drGMMMethodResult, len(goResults))
			for _, method := range goResults {
				goByName[method.Name] = method
			}
			pyByName := make(map[string]drGMMMethodResult, len(pythonResult.methods))
			for _, method := range pythonResult.methods {
				pyByName[method.Name] = method
			}

			methodOrder := []string{"PCA", "t-SNE", "UMAP"}
			minARI := map[string]float64{
				"PCA":   0.50,
				"t-SNE": 0.15,
				"UMAP":  0.20,
			}
			for _, methodName := range methodOrder {
				goMethod, ok := goByName[methodName]
				if !ok {
					t.Fatalf("missing go method result: %s", methodName)
				}
				pyMethod, ok := pyByName[methodName]
				if !ok {
					t.Fatalf("missing python method result: %s", methodName)
				}

				validateDRGMMResult(t, methodName+"(go)", goMethod, len(vectors))
				validateDRGMMResult(t, methodName+"(python)", pyMethod, len(vectors))

				ari := numutil.AdjustedRandIndex(goMethod.Labels, pyMethod.Labels)
				if math.IsNaN(ari) {
					t.Fatalf("%s ari(go,python) is NaN", methodName)
				}
				if ari < minARI[methodName] {
					t.Fatalf(
						"%s ari(go,python) too low: got %.4f want >= %.4f",
						methodName,
						ari,
						minARI[methodName],
					)
				}

				trustDiff := math.Abs(goMethod.Trustworthiness - pyMethod.Trustworthiness)
				distDiff := math.Abs(goMethod.DistanceCorrelation - pyMethod.DistanceCorrelation)
				if trustDiff > 0.4 {
					t.Fatalf(
						"%s trustworthiness diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.Trustworthiness,
						pyMethod.Trustworthiness,
						trustDiff,
					)
				}
				if distDiff > 0.45 {
					t.Fatalf(
						"%s distance correlation diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.DistanceCorrelation,
						pyMethod.DistanceCorrelation,
						distDiff,
					)
				}

				t.Logf(
					"%s: ari(go,py)=%.4f trust_diff=%.4f distcorr_diff=%.4f bic(go)=%.2f bic(py)=%.2f",
					methodName,
					ari,
					trustDiff,
					distDiff,
					goMethod.BIC,
					pyMethod.BIC,
				)
			}

			renderDRGMMGoVsPythonPlotViaUV(
				t,
				drGMMGoVsPythonPlotPayload{
					DatasetName:   tc.Name,
					GoMethods:     goResults,
					PythonMethods: pythonResult.methods,
					OutputFile: filepath.Join(
						"testdata",
						fmt.Sprintf("dr_gmm_compare_go_vs_python_%s.png", tc.Name),
					),
				},
			)
		})
		if !ok {
			allPassed = false
		}
	}

	if !allPassed {
		t.Logf("skip stitching dr+gmm overview because one or more dataset subtests failed")
		return
	}

	stitchDRGMMPlotsViaUV(
		t,
		drGMMStitchPayload{
			Title: "Go vs Python: PCA/t-SNE/UMAP + GaussianMixture",
			Datasets: []drGMMStitchDataset{
				{
					Name:      "embeddings_1",
					ImageFile: filepath.Join("testdata", "dr_gmm_compare_go_vs_python_embeddings_1.png"),
				},
				{
					Name:      "embeddings_2",
					ImageFile: filepath.Join("testdata", "dr_gmm_compare_go_vs_python_embeddings_2.png"),
				},
				{
					Name:      "embeddings_3",
					ImageFile: filepath.Join("testdata", "dr_gmm_compare_go_vs_python_embeddings_3.png"),
				},
			},
			OutputFile: filepath.Join("testdata", "dr_gmm_compare_overview.png"),
		},
	)
}

func resolveDRGMMClusterCount(nSamples int) int {
	if nSamples <= 1 {
		return 1
	}
	clusterCount := 4
	if clusterCount > nSamples {
		clusterCount = nSamples
	}
	if clusterCount < 2 && nSamples >= 2 {
		clusterCount = 2
	}
	return clusterCount
}

func buildGoDRGMMMethodResult(
	methodName string,
	high [][]float64,
	embedding [][]float64,
	normalizedHighCondensed []float64,
	trustNeighbors int,
	clusterCount int,
	seed uint64,
) (drGMMMethodResult, error) {
	gmmModel, err := mixture.NewGaussianMixture(
		clusterCount,
		mixture.WithTolerance(1e-3),
		mixture.WithRegularization(1e-6),
		mixture.WithMaxIterations(100),
		mixture.WithNInit(1),
		mixture.WithInitParams(mixture.InitParamsKMeans),
		mixture.WithRandomSeed(seed),
	)
	if err != nil {
		return drGMMMethodResult{}, err
	}

	evaluation, err := gmmModel.FitEvaluate(embedding)
	if err != nil {
		return drGMMMethodResult{}, err
	}

	lowCondensed := numutil.CondensedPairwiseDistances(embedding)
	normalizedLow := numutil.NormalizeByMean(lowCondensed)
	neighbors := trustNeighbors
	if neighbors >= len(high) {
		neighbors = max(1, len(high)-1)
	}

	result := drGMMMethodResult{
		Name:                methodName,
		Embedding:           numutil.Clone2DFloat64(embedding),
		Labels:              slices.Clone(evaluation.Labels),
		Means:               evaluation.Means,
		Trustworthiness:     numutil.Trustworthiness(high, embedding, neighbors),
		DistanceCorrelation: numutil.PearsonCorrelation(normalizedHighCondensed, normalizedLow),
		LogLikelihood:       evaluation.LogLikelihood,
		BIC:                 evaluation.BIC,
	}
	canonicalizeDRGMMMethodResult(&result)
	return result, nil
}

func validateDRGMMResult(t *testing.T, name string, result drGMMMethodResult, expectedSamples int) {
	t.Helper()
	if len(result.Embedding) != expectedSamples {
		t.Fatalf("%s embedding rows mismatch: got=%d want=%d", name, len(result.Embedding), expectedSamples)
	}
	if len(result.Labels) != expectedSamples {
		t.Fatalf("%s labels mismatch: got=%d want=%d", name, len(result.Labels), expectedSamples)
	}
	if len(result.Embedding) > 0 && len(result.Embedding[0]) != 2 {
		t.Fatalf("%s embedding dim should be 2", name)
	}
	if !isFinite(result.Trustworthiness) || result.Trustworthiness < 0 || result.Trustworthiness > 1 {
		t.Fatalf("%s trustworthiness out of range: %.6f", name, result.Trustworthiness)
	}
	if !isFinite(result.DistanceCorrelation) || result.DistanceCorrelation < -1 || result.DistanceCorrelation > 1 {
		t.Fatalf("%s distance correlation out of range: %.6f", name, result.DistanceCorrelation)
	}
	if !isFinite(result.LogLikelihood) {
		t.Fatalf("%s log_likelihood is not finite: %.6f", name, result.LogLikelihood)
	}
	if !isFinite(result.BIC) {
		t.Fatalf("%s bic is not finite: %.6f", name, result.BIC)
	}
}

func canonicalizeDRGMMMethodResult(result *drGMMMethodResult) {
	if len(result.Means) == 0 {
		return
	}

	order := make([]int, len(result.Means))
	for idx := range order {
		order[idx] = idx
	}
	sort.Slice(order, func(i, j int) bool {
		left := result.Means[order[i]]
		right := result.Means[order[j]]
		for dim := 0; dim < min(len(left), len(right)); dim++ {
			if left[dim] != right[dim] {
				return left[dim] < right[dim]
			}
		}
		return len(left) < len(right)
	})

	remap := make([]int, len(order))
	for newIdx, oldIdx := range order {
		remap[oldIdx] = newIdx
	}

	orderedMeans := make([][]float64, len(order))
	for newIdx, oldIdx := range order {
		orderedMeans[newIdx] = slices.Clone(result.Means[oldIdx])
	}
	orderedLabels := make([]int, len(result.Labels))
	for sampleIdx, label := range result.Labels {
		if label >= 0 && label < len(remap) {
			orderedLabels[sampleIdx] = remap[label]
		} else {
			orderedLabels[sampleIdx] = label
		}
	}

	result.Means = orderedMeans
	result.Labels = orderedLabels
}

func loadPythonDRGMMSnapshotViaUV(
	vectors [][]float64,
	tsnePerplexity float64,
	trustNeighbors int,
	clusterCount int,
	seed int,
) ([]drGMMMethodResult, string, error) {
	request := drGMMPythonSnapshotRequest{
		Vectors:        vectors,
		TSNEPerplexity: tsnePerplexity,
		TrustNeighbors: trustNeighbors,
		ClusterCount:   clusterCount,
		Seed:           seed,
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drGMMUVPythonPath,
		"python",
		"dr_gmm_compare.py",
		"--snapshot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, stderr.String(), err
	}

	var response drGMMPythonSnapshotResponse
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
		return nil, stderr.String(), fmt.Errorf("decode python dr+gmm snapshot failed: %w", err)
	}
	for idx := range response.Methods {
		canonicalizeDRGMMMethodResult(&response.Methods[idx])
	}
	return response.Methods, stderr.String(), nil
}

func renderDRGMMGoVsPythonPlotViaUV(t *testing.T, payload drGMMGoVsPythonPlotPayload) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dr+gmm go-vs-python payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drGMMUVPythonPath,
		"python",
		"dr_gmm_compare.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render dr+gmm go-vs-python plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python dr+gmm plot output: %s", output)
	}
}

func stitchDRGMMPlotsViaUV(t *testing.T, payload drGMMStitchPayload) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dr+gmm stitch payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drGMMUVPythonPath,
		"python",
		"dr_gmm_compare.py",
		"--stitch-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("stitch dr+gmm plots failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python dr+gmm stitch output: %s", output)
	}
}

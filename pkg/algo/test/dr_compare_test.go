package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/decomposition"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

const (
	drCompareUVPythonPath = "../.venv/bin/python"
	drFeatureLimit        = 24
	drTrustNeighbors      = 10
)

type drDatasetCase struct {
	Name string
	File string
}

type drMethodResult struct {
	Name                string      `json:"name"`
	Embedding           [][]float64 `json:"embedding"`
	Trustworthiness     float64     `json:"trustworthiness"`
	DistanceCorrelation float64     `json:"distance_correlation"`
}

type drPlotPayload struct {
	DatasetName string           `json:"dataset_name"`
	Methods     []drMethodResult `json:"methods"`
	OutputFile  string           `json:"output_file,omitempty"`
}

type drGoVsPythonPlotPayload struct {
	DatasetName   string           `json:"dataset_name"`
	GoMethods     []drMethodResult `json:"go_methods"`
	PythonMethods []drMethodResult `json:"python_methods"`
	OutputFile    string           `json:"output_file,omitempty"`
}

type drPythonSnapshotRequest struct {
	Vectors        [][]float64 `json:"vectors"`
	TSNEPerplexity float64     `json:"tsne_perplexity"`
	TrustNeighbors int         `json:"trust_neighbors"`
}

type drPythonSnapshotResponse struct {
	Methods []drMethodResult `json:"methods"`
}

type drPythonSnapshotResult struct {
	methods []drMethodResult
	stderr  string
	err     error
}

type drStitchDataset struct {
	Name      string `json:"name"`
	ImageFile string `json:"image_file"`
}

type drStitchPayload struct {
	Title      string            `json:"title,omitempty"`
	Datasets   []drStitchDataset `json:"datasets"`
	OutputFile string            `json:"output_file,omitempty"`
}

func TestDimensionalityReduction_ComparePCA_TSNE_UMAP(t *testing.T) {
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
			vectors = truncateFeatureDim(vectors, drFeatureLimit)
			tsnePerplexity := resolvePerplexity(len(vectors))

			pythonResultCh := make(chan drPythonSnapshotResult, 1)
			go func() {
				methods, stderr, err := loadPythonMethodsViaUV(
					vectors,
					tsnePerplexity,
					drTrustNeighbors,
				)
				pythonResultCh <- drPythonSnapshotResult{
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
				manifold.WithRandomSeed(42),
			)
			if err != nil {
				t.Fatalf("new tsne failed: %v", err)
			}
			tsneEmbedding, err := tsne.FitTransform(vectors)
			if err != nil {
				t.Fatalf("tsne fit_transform failed: %v", err)
			}

			umap, err := manifold.NewUMAP(
				2,
				manifold.WithUMAPNNeighbors(15),
				manifold.WithUMAPMinDist(0.1),
				manifold.WithUMAPSpread(1.0),
				manifold.WithUMAPMetric("euclidean"),
				manifold.WithUMAPInit(manifold.UMAPInitSpectral),
				manifold.WithUMAPRandomSeed(42),
			)
			if err != nil {
				t.Fatalf("new umap failed: %v", err)
			}
			umapEmbedding, err := umap.FitTransform(vectors)
			if err != nil {
				t.Fatalf("umap fit_transform failed: %v", err)
			}

			results := []drMethodResult{
				buildMethodResult("PCA", vectors, pcaEmbedding, highNormalized),
				buildMethodResult("t-SNE", vectors, tsneEmbedding, highNormalized),
				buildMethodResult("UMAP", vectors, umapEmbedding, highNormalized),
			}

			for _, result := range results {
				if len(result.Embedding) != len(vectors) {
					t.Fatalf(
						"%s embedding row mismatch: got=%d want=%d",
						result.Name,
						len(result.Embedding),
						len(vectors),
					)
				}
				if len(result.Embedding) == 0 || len(result.Embedding[0]) != 2 {
					t.Fatalf("%s embedding should have 2 dims", result.Name)
				}
				if !isFinite(result.Trustworthiness) || result.Trustworthiness < 0 || result.Trustworthiness > 1 {
					t.Fatalf("%s trustworthiness out of range: %.6f", result.Name, result.Trustworthiness)
				}
				if !isFinite(result.DistanceCorrelation) || result.DistanceCorrelation < -1 || result.DistanceCorrelation > 1 {
					t.Fatalf("%s distance correlation out of range: %.6f", result.Name, result.DistanceCorrelation)
				}
				t.Logf(
					"%s: trustworthiness=%.4f distance_correlation=%.4f",
					result.Name,
					result.Trustworthiness,
					result.DistanceCorrelation,
				)
			}

			pythonResult := <-pythonResultCh
			if pythonResult.err != nil {
				t.Fatalf(
					"run python dr snapshot via uv failed: %v\nstderr:\n%s",
					pythonResult.err,
					pythonResult.stderr,
				)
			}

			pythonByName := make(map[string]drMethodResult, len(pythonResult.methods))
			for _, method := range pythonResult.methods {
				pythonByName[method.Name] = method
			}
			goByName := make(map[string]drMethodResult, len(results))
			for _, method := range results {
				goByName[method.Name] = method
			}

			methodOrder := []string{"PCA", "t-SNE", "UMAP"}
			for _, methodName := range methodOrder {
				goMethod, ok := goByName[methodName]
				if !ok {
					t.Fatalf("missing go method result: %s", methodName)
				}
				pyMethod, ok := pythonByName[methodName]
				if !ok {
					t.Fatalf("missing python method result: %s", methodName)
				}

				if len(pyMethod.Embedding) != len(vectors) {
					t.Fatalf(
						"%s python embedding row mismatch: got=%d want=%d",
						methodName,
						len(pyMethod.Embedding),
						len(vectors),
					)
				}

				trustDiff := math.Abs(goMethod.Trustworthiness - pyMethod.Trustworthiness)
				distCorrDiff := math.Abs(goMethod.DistanceCorrelation - pyMethod.DistanceCorrelation)
				if trustDiff > 0.35 {
					t.Fatalf(
						"%s trustworthiness diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.Trustworthiness,
						pyMethod.Trustworthiness,
						trustDiff,
					)
				}
				if distCorrDiff > 0.40 {
					t.Fatalf(
						"%s distance correlation diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.DistanceCorrelation,
						pyMethod.DistanceCorrelation,
						distCorrDiff,
					)
				}

				goNorm := numutil.NormalizeByMean(numutil.CondensedPairwiseDistances(goMethod.Embedding))
				pyNorm := numutil.NormalizeByMean(numutil.CondensedPairwiseDistances(pyMethod.Embedding))
				structureCorr := numutil.PearsonCorrelation(goNorm, pyNorm)
				t.Logf(
					"%s Go vs Python: trust_diff=%.4f dist_corr_diff=%.4f structure_corr=%.4f",
					methodName,
					trustDiff,
					distCorrDiff,
					structureCorr,
				)
			}

			renderGoVsPythonPlotViaUV(
				t,
				drGoVsPythonPlotPayload{
					DatasetName:   tc.Name,
					GoMethods:     results,
					PythonMethods: pythonResult.methods,
					OutputFile: filepath.Join(
						"testdata",
						fmt.Sprintf("dr_compare_go_vs_python_%s.png", tc.Name),
					),
				},
			)
		})
		if !ok {
			allPassed = false
		}
	}

	if !allPassed {
		t.Logf("skip stitching overview image because one or more dataset subtests failed")
		return
	}

	stitchPayload := drStitchPayload{
		Title: "Go vs Python: PCA / t-SNE / UMAP (all datasets)",
		Datasets: []drStitchDataset{
			{Name: "embeddings_1", ImageFile: filepath.Join("testdata", "dr_compare_go_vs_python_embeddings_1.png")},
			{Name: "embeddings_2", ImageFile: filepath.Join("testdata", "dr_compare_go_vs_python_embeddings_2.png")},
			{Name: "embeddings_3", ImageFile: filepath.Join("testdata", "dr_compare_go_vs_python_embeddings_3.png")},
		},
		OutputFile: filepath.Join("testdata", "dr_compare_overview.png"),
	}
	stitchComparisonPlotsViaUV(t, stitchPayload)
}

func buildMethodResult(
	methodName string,
	high [][]float64,
	embedding [][]float64,
	normalizedHighCondensed []float64,
) drMethodResult {
	lowCondensed := numutil.CondensedPairwiseDistances(embedding)
	normalizedLow := numutil.NormalizeByMean(lowCondensed)

	return drMethodResult{
		Name:                methodName,
		Embedding:           numutil.Clone2DFloat64(embedding),
		Trustworthiness:     numutil.Trustworthiness(high, embedding, drTrustNeighbors),
		DistanceCorrelation: numutil.PearsonCorrelation(normalizedHighCondensed, normalizedLow),
	}
}

func loadJSONLVectors(t *testing.T, path string) [][]float64 {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl failed: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	vectors := make([][]float64, 0, 512)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var vector []float64
		if err := json.Unmarshal([]byte(line), &vector); err != nil {
			t.Fatalf("unmarshal jsonl vector failed: %v", err)
		}
		vectors = append(vectors, vector)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl failed: %v", err)
	}
	if len(vectors) == 0 {
		t.Fatalf("jsonl dataset has no vectors: %s", path)
	}
	return vectors
}

func truncateFeatureDim(vectors [][]float64, featureLimit int) [][]float64 {
	return numutil.TruncateFeatureDim(vectors, featureLimit)
}

func loadPythonMethodsViaUV(
	vectors [][]float64,
	tsnePerplexity float64,
	trustNeighbors int,
) ([]drMethodResult, string, error) {
	request := drPythonSnapshotRequest{
		Vectors:        vectors,
		TSNEPerplexity: tsnePerplexity,
		TrustNeighbors: trustNeighbors,
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drCompareUVPythonPath,
		"python",
		"dr_compare.py",
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

	var response drPythonSnapshotResponse
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
		return nil, stderr.String(), fmt.Errorf("decode python dr snapshot failed: %w", err)
	}
	return response.Methods, stderr.String(), nil
}

func renderComparisonPlotViaUV(t *testing.T, payload drPlotPayload) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dr plot payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drCompareUVPythonPath,
		"python",
		"dr_compare.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render dr comparison plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python plot output: %s", output)
	}
}

func renderGoVsPythonPlotViaUV(t *testing.T, payload drGoVsPythonPlotPayload) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dr go-vs-python payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drCompareUVPythonPath,
		"python",
		"dr_compare.py",
		"--plot-go-vs-python-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render dr go-vs-python plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python go-vs-python plot output: %s", output)
	}
}

func stitchComparisonPlotsViaUV(t *testing.T, payload drStitchPayload) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dr stitch payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		drCompareUVPythonPath,
		"python",
		"dr_compare.py",
		"--stitch-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("stitch dr comparison plots failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python stitch output: %s", output)
	}
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func resolvePerplexity(nSamples int) float64 {
	if nSamples <= 2 {
		return 1
	}
	upper := float64(nSamples - 1)
	target := min(30.0, math.Max(5, float64(nSamples)/3.0))
	if target >= upper {
		target = math.Max(1, upper-1)
	}
	if target < 1 {
		target = 1
	}
	return target
}

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
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/decomposition"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	algonormalize "github.com/gonotelm-lab/gonotelm/pkg/algo/normalize"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

type drNormAutoGMMPythonRequest struct {
	Vectors        [][]float64 `json:"vectors"`
	TSNEPerplexity float64     `json:"tsne_perplexity"`
	TrustNeighbors int         `json:"trust_neighbors"`
	Seed           int         `json:"seed"`
	AutoCluster    bool        `json:"auto_cluster"`
	NormalizeL2    bool        `json:"normalize_l2"`
}

func TestIntegration_NormalizeThenDRAutoGMM_CompareGoAndPython(t *testing.T) {
	cases := []drDatasetCase{
		{Name: "embeddings_1", File: "embeddings_1.jsonl"},
		{Name: "embeddings_2", File: "embeddings_2.jsonl"},
		{Name: "embeddings_3", File: "embeddings_3.jsonl"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			vectors := loadJSONLVectors(t, filepath.Join("testdata", tc.File))
			vectors = truncateFeatureDim(vectors, drGMMFeatureLimit)
			normalizedVectors, err := algonormalize.L2(vectors)
			if err != nil {
				t.Fatalf("l2 normalize failed: %v", err)
			}
			tsnePerplexity := resolvePerplexity(len(normalizedVectors))

			pythonResultCh := make(chan drGMMPythonSnapshotResult, 1)
			go func() {
				methods, stderr, err := loadPythonNormAutoDRGMMSnapshotViaUV(
					normalizedVectors,
					tsnePerplexity,
					drTrustNeighbors,
					drGMMSeed,
				)
				pythonResultCh <- drGMMPythonSnapshotResult{
					methods: methods,
					stderr:  stderr,
					err:     err,
				}
			}()

			highCondensed := numutil.CondensedPairwiseDistances(normalizedVectors)
			highNormalized := numutil.NormalizeByMean(highCondensed)

			pca := decomposition.NewPCA(2)
			pcaEmbedding, err := pca.FitTransform(normalizedVectors)
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
			tsneEmbedding, err := tsne.FitTransform(normalizedVectors)
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
			umapEmbedding, err := umapModel.FitTransform(normalizedVectors)
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
				result, err := buildGoAutoDRGMMMethodResult(
					method.name,
					normalizedVectors,
					method.embedding,
					highNormalized,
					drTrustNeighbors,
					drGMMSeed,
				)
				if err != nil {
					t.Fatalf("build go auto method result failed for %s: %v", method.name, err)
				}
				goResults = append(goResults, result)
			}

			pythonResult := <-pythonResultCh
			if pythonResult.err != nil {
				t.Fatalf(
					"run python normalized dr+auto-gmm snapshot via uv failed: %v\nstderr:\n%s",
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
			for _, methodName := range methodOrder {
				goMethod, ok := goByName[methodName]
				if !ok {
					t.Fatalf("missing go method result: %s", methodName)
				}
				pyMethod, ok := pyByName[methodName]
				if !ok {
					t.Fatalf("missing python method result: %s", methodName)
				}

				validateDRGMMResult(t, methodName+"(go)", goMethod, len(normalizedVectors))
				validateDRGMMResult(t, methodName+"(python)", pyMethod, len(normalizedVectors))

				ari := numutil.AdjustedRandIndex(goMethod.Labels, pyMethod.Labels)
				if math.IsNaN(ari) {
					t.Fatalf("%s ari(go,python) is NaN", methodName)
				}
				if ari < 0.10 {
					t.Logf("%s warning: low ari(go,python)=%.4f", methodName, ari)
				}

				trustDiff := math.Abs(goMethod.Trustworthiness - pyMethod.Trustworthiness)
				distDiff := math.Abs(goMethod.DistanceCorrelation - pyMethod.DistanceCorrelation)
				if trustDiff > 0.45 {
					t.Fatalf(
						"%s trustworthiness diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.Trustworthiness,
						pyMethod.Trustworthiness,
						trustDiff,
					)
				}
				if distDiff > 0.50 {
					t.Fatalf(
						"%s distance correlation diff too large: go=%.4f py=%.4f diff=%.4f",
						methodName,
						goMethod.DistanceCorrelation,
						pyMethod.DistanceCorrelation,
						distDiff,
					)
				}

				t.Logf(
					"%s(norm+auto): ari(go,py)=%.4f trust_diff=%.4f distcorr_diff=%.4f k(go)=%d k(py)=%d",
					methodName,
					ari,
					trustDiff,
					distDiff,
					len(goMethod.Means),
					len(pyMethod.Means),
				)
			}

			renderDRGMMGoVsPythonPlotViaUV(
				t,
				drGMMGoVsPythonPlotPayload{
					DatasetName:   tc.Name + "_norm_auto",
					GoMethods:     goResults,
					PythonMethods: pythonResult.methods,
					OutputFile: filepath.Join(
						"testdata",
						fmt.Sprintf("dr_norm_auto_gmm_compare_go_vs_python_%s.png", tc.Name),
					),
				},
			)
		})
	}
}

func buildGoAutoDRGMMMethodResult(
	methodName string,
	high [][]float64,
	embedding [][]float64,
	normalizedHighCondensed []float64,
	trustNeighbors int,
	seed uint64,
) (drGMMMethodResult, error) {
	gmmModel, evaluation, _, err := mixture.AutoSelectGaussianMixtureDefault(
		embedding,
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
	_ = gmmModel

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

func loadPythonNormAutoDRGMMSnapshotViaUV(
	vectors [][]float64,
	tsnePerplexity float64,
	trustNeighbors int,
	seed int,
) ([]drGMMMethodResult, string, error) {
	request := drNormAutoGMMPythonRequest{
		Vectors:        vectors,
		TSNEPerplexity: tsnePerplexity,
		TrustNeighbors: trustNeighbors,
		Seed:           seed,
		AutoCluster:    true,
		NormalizeL2:    false,
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
		return nil, stderr.String(), fmt.Errorf("decode python normalized dr+auto-gmm snapshot failed: %w", err)
	}
	for idx := range response.Methods {
		canonicalizeDRGMMMethodResult(&response.Methods[idx])
	}
	return response.Methods, stderr.String(), nil
}

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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

const (
	casesFileName = "gmm_cases.json"
	uvPythonPath  = "../../.venv/bin/python"
)

type gmmCase struct {
	Name             string  `json:"name"`
	Kind             string  `json:"kind"` // payload_json or jsonl
	SourceFile       string  `json:"source_file"`
	ClusterCount     int     `json:"cluster_count"`
	MaxIterations    int     `json:"max_iterations"`
	Tolerance        float64 `json:"tolerance"`
	Regularization   float64 `json:"regularization"`
	Seed             uint64  `json:"seed"`
	FeatureLimit     int     `json:"feature_limit"`
	MinARIToSklearn  float64 `json:"min_ari_to_sklearn"`
	MinARIToTrue     float64 `json:"min_ari_to_true"`
	LogLikelihoodTol float64 `json:"log_likelihood_tolerance"`
	BICTol           float64 `json:"bic_tolerance"`
	InitParams       string  `json:"init_params"`
	CovarianceType   string  `json:"covariance_type"`
	RequestedNInit   int     `json:"n_init"`
	RequestedPlotDim int     `json:"plot_dim"`
}

type gmmSnapshot struct {
	Name               string      `json:"name"`
	DatasetFile        string      `json:"dataset_file"`
	CovarianceType     string      `json:"covariance_type"`
	ClusterCount       int         `json:"cluster_count"`
	Labels             []int       `json:"labels"`
	Weights            []float64   `json:"weights"`
	Means              [][]float64 `json:"means"`
	LogLikelihood      float64     `json:"log_likelihood"`
	BIC                float64     `json:"bic"`
	Iterations         int         `json:"iterations"`
	Converged          bool        `json:"converged"`
	ARIVsTrueLabels    *float64    `json:"ari_vs_true_labels,omitempty"`
	ARIVsSklearnLabels float64     `json:"ari_vs_sklearn_labels,omitempty"`
}

type payloadDataset struct {
	Name               string      `json:"name"`
	Mode               string      `json:"mode"`
	Vectors            [][]float64 `json:"vectors"`
	TrueLabels         []int       `json:"true_labels"`
	ClusterCount       int         `json:"cluster_count"`
	MaxIterations      int         `json:"max_iterations"`
	Tolerance          float64     `json:"tolerance"`
	Regularization     float64     `json:"regularization"`
	Seed               uint64      `json:"seed"`
	RequestedClusterID int         `json:"requested_cluster_count"`
}

type datasetInput struct {
	Vectors    [][]float64
	TrueLabels []int
}

func TestGaussianMixture_MatchesSklearnBaselines(t *testing.T) {
	testdataDir := "testdata"
	caseFilePath := filepath.Join(testdataDir, casesFileName)
	cases := loadCases(t, caseFilePath)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			data := loadDatasetForCase(t, testdataDir, tc)

			pythonResultCh := make(chan sklearnSnapshotResult, 1)
			go func() {
				snapshot, stderr, err := loadSklearnSnapshotViaUV(tc.Name)
				pythonResultCh <- sklearnSnapshotResult{
					snapshot: snapshot,
					stderr:   stderr,
					err:      err,
				}
			}()

			model, err := mixture.NewGaussianMixture(
				tc.ClusterCount,
				mixture.WithTolerance(tc.Tolerance),
				mixture.WithRegularization(tc.Regularization),
				mixture.WithMaxIterations(tc.MaxIterations),
				mixture.WithNInit(max(tc.RequestedNInit, 1)),
				mixture.WithInitParams(parseInitParams(tc.InitParams)),
				mixture.WithRandomSeed(tc.Seed),
			)
			if err != nil {
				t.Fatalf("new gaussian mixture failed: %v", err)
			}

			evaluation, err := model.FitEvaluate(data.Vectors)
			if err != nil {
				t.Fatalf("fit_evaluate failed: %v", err)
			}

			goSnapshot := gmmSnapshot{
				Name:           tc.Name,
				DatasetFile:    tc.SourceFile,
				CovarianceType: defaultIfEmpty(tc.CovarianceType, "full"),
				ClusterCount:   tc.ClusterCount,
				Labels:         slices.Clone(evaluation.Labels),
				Weights:        evaluation.Weights,
				Means:          evaluation.Means,
				LogLikelihood:  evaluation.LogLikelihood,
				BIC:            evaluation.BIC,
				Iterations:     evaluation.Iterations,
				Converged:      evaluation.Converged,
			}
			if len(data.TrueLabels) > 0 {
				ariTrue := numutil.AdjustedRandIndex(evaluation.Labels, data.TrueLabels)
				goSnapshot.ARIVsTrueLabels = &ariTrue
			}

			pythonResult := <-pythonResultCh
			if pythonResult.err != nil {
				t.Fatalf(
					"run sklearn snapshot via uv failed: %v\nstderr:\n%s",
					pythonResult.err,
					pythonResult.stderr,
				)
			}
			sklearnSnapshot := pythonResult.snapshot
			canonicalizeSnapshot(&sklearnSnapshot)

			canonicalizeSnapshot(&goSnapshot)
			goSnapshot.ARIVsSklearnLabels = numutil.AdjustedRandIndex(goSnapshot.Labels, sklearnSnapshot.Labels)

			if tc.MinARIToSklearn > 0 {
				if goSnapshot.ARIVsSklearnLabels < tc.MinARIToSklearn {
					t.Fatalf(
						"ari(go, sklearn) too low: got %.8f, want >= %.8f",
						goSnapshot.ARIVsSklearnLabels,
						tc.MinARIToSklearn,
					)
				}
			}

			if goSnapshot.ARIVsTrueLabels != nil {
				minARIToTrue := tc.MinARIToTrue
				if minARIToTrue <= 0 {
					minARIToTrue = 0.995
				}
				if *goSnapshot.ARIVsTrueLabels < minARIToTrue {
					t.Fatalf(
						"ari(go, true_labels) too low: got %.8f, want >= %.8f",
						*goSnapshot.ARIVsTrueLabels,
						minARIToTrue,
					)
				}
			}

			logLikelihoodTol := tc.LogLikelihoodTol
			if logLikelihoodTol <= 0 {
				logLikelihoodTol = scaleTolerance(sklearnSnapshot.LogLikelihood, 5e-5, 5e-3)
			}
			bicTol := tc.BICTol
			if bicTol <= 0 {
				bicTol = scaleTolerance(sklearnSnapshot.BIC, 5e-5, 5e-2)
			}

			assertCloseFloat(
				t,
				"log_likelihood",
				goSnapshot.LogLikelihood,
				sklearnSnapshot.LogLikelihood,
				logLikelihoodTol,
			)
			assertCloseFloat(
				t,
				"bic",
				goSnapshot.BIC,
				sklearnSnapshot.BIC,
				bicTol,
			)

			renderComparisonPlotViaUV(
				t,
				gmmComparePlotRequest{
					CaseName:        tc.Name,
					Vectors:         data.Vectors,
					SklearnSnapshot: sklearnSnapshot,
					GoSnapshot:      goSnapshot,
					OutputFile: filepath.Join(
						testdataDir,
						fmt.Sprintf("gmm_compare_%s.png", tc.Name),
					),
				},
			)
		})
	}
}

type sklearnSnapshotResult struct {
	snapshot gmmSnapshot
	stderr   string
	err      error
}

func loadCases(t *testing.T, path string) []gmmCase {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cases failed: %v", err)
	}

	var cases []gmmCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("unmarshal cases failed: %v", err)
	}
	if len(cases) == 0 {
		t.Fatalf("no gmm test cases found in %s", path)
	}

	for idx, tc := range cases {
		if tc.Name == "" {
			t.Fatalf("case[%d] has empty name", idx)
		}
		if tc.SourceFile == "" {
			t.Fatalf("case[%d] has empty source_file", idx)
		}
		if tc.ClusterCount <= 0 {
			t.Fatalf("case[%d] has invalid cluster_count=%d", idx, tc.ClusterCount)
		}
		if tc.MaxIterations <= 0 {
			t.Fatalf("case[%d] has invalid max_iterations=%d", idx, tc.MaxIterations)
		}
		if tc.Kind != "payload_json" && tc.Kind != "jsonl" {
			t.Fatalf("case[%d] has unsupported kind=%q", idx, tc.Kind)
		}
	}

	return cases
}

func loadDatasetForCase(t *testing.T, testdataDir string, tc gmmCase) datasetInput {
	t.Helper()

	sourcePath := filepath.Join(testdataDir, tc.SourceFile)
	switch tc.Kind {
	case "payload_json":
		payload := loadPayloadDataset(t, sourcePath)
		vectors := truncateFeatureDim(payload.Vectors, tc.FeatureLimit)
		return datasetInput{
			Vectors:    vectors,
			TrueLabels: slices.Clone(payload.TrueLabels),
		}
	case "jsonl":
		vectors := loadJSONLVectors(t, sourcePath)
		vectors = truncateFeatureDim(vectors, tc.FeatureLimit)
		return datasetInput{
			Vectors: vectors,
		}
	default:
		t.Fatalf("unsupported case kind %q", tc.Kind)
		return datasetInput{}
	}
}

func loadPayloadDataset(t *testing.T, path string) payloadDataset {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload dataset failed: %v", err)
	}

	var payload payloadDataset
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload dataset failed: %v", err)
	}
	if len(payload.Vectors) == 0 {
		t.Fatalf("payload dataset has no vectors: %s", path)
	}
	return payload
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

func loadSklearnSnapshotViaUV(caseName string) (gmmSnapshot, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"gmm_test.py",
		"--case",
		caseName,
		"--snapshot-stdout",
	)
	cmd.Dir = "."

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return gmmSnapshot{}, stderr.String(), err
	}

	var snapshot gmmSnapshot
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &snapshot); err != nil {
		return gmmSnapshot{}, stderr.String(), fmt.Errorf("decode sklearn snapshot json failed: %w", err)
	}
	return snapshot, stderr.String(), nil
}

type gmmComparePlotRequest struct {
	CaseName        string      `json:"case_name"`
	Vectors         [][]float64 `json:"vectors"`
	SklearnSnapshot gmmSnapshot `json:"sklearn_snapshot"`
	GoSnapshot      gmmSnapshot `json:"go_snapshot"`
	OutputFile      string      `json:"output_file,omitempty"`
}

func renderComparisonPlotViaUV(t *testing.T, request gmmComparePlotRequest) {
	t.Helper()

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal plot request failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"gmm_test.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render comparison plot via uv failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python plot output: %s", output)
	}
}

func canonicalizeSnapshot(snapshot *gmmSnapshot) {
	if len(snapshot.Means) == 0 {
		return
	}

	order := make([]int, len(snapshot.Means))
	for idx := range order {
		order[idx] = idx
	}
	slices.SortFunc(order, func(a, b int) int {
		for featureIdx := range snapshot.Means[a] {
			if snapshot.Means[a][featureIdx] < snapshot.Means[b][featureIdx] {
				return -1
			}
			if snapshot.Means[a][featureIdx] > snapshot.Means[b][featureIdx] {
				return 1
			}
		}
		return 0
	})

	remap := make([]int, len(order))
	for newIdx, oldIdx := range order {
		remap[oldIdx] = newIdx
	}

	orderedWeights := make([]float64, len(snapshot.Weights))
	orderedMeans := make([][]float64, len(snapshot.Means))
	for newIdx, oldIdx := range order {
		orderedWeights[newIdx] = snapshot.Weights[oldIdx]
		orderedMeans[newIdx] = slices.Clone(snapshot.Means[oldIdx])
	}

	orderedLabels := make([]int, len(snapshot.Labels))
	for sampleIdx, label := range snapshot.Labels {
		if label < 0 || label >= len(remap) {
			orderedLabels[sampleIdx] = label
			continue
		}
		orderedLabels[sampleIdx] = remap[label]
	}

	snapshot.Weights = orderedWeights
	snapshot.Means = orderedMeans
	snapshot.Labels = orderedLabels
}

func parseInitParams(value string) mixture.InitParams {
	switch value {
	case "", "kmeans":
		return mixture.InitParamsKMeans
	case "random":
		return mixture.InitParamsRandom
	default:
		return mixture.InitParamsKMeans
	}
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func scaleTolerance(reference, relative, absolute float64) float64 {
	return math.Abs(reference)*relative + absolute
}

func assertCloseMatrix(t *testing.T, name string, got, want [][]float64, tolerance float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s row count mismatch: got=%d want=%d", name, len(got), len(want))
	}
	for rowIdx := range got {
		assertCloseVector(t, fmt.Sprintf("%s[%d]", name, rowIdx), got[rowIdx], want[rowIdx], tolerance)
	}
}

func assertCloseVector(t *testing.T, name string, got, want []float64, tolerance float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got=%d want=%d", name, len(got), len(want))
	}
	for idx := range got {
		assertCloseFloat(t, fmt.Sprintf("%s[%d]", name, idx), got[idx], want[idx], tolerance)
	}
}

func assertCloseFloat(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()

	diff := math.Abs(got - want)
	if diff > tolerance {
		t.Fatalf(
			"%s mismatch: got=%.10f want=%.10f diff=%.10f tol=%.10f",
			name,
			got,
			want,
			diff,
			tolerance,
		)
	}
}

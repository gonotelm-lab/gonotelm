package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

type umapParams struct {
	NComponents        int     `json:"n_components"`
	NNeighbors         int     `json:"n_neighbors"`
	MinDist            float64 `json:"min_dist"`
	Spread             float64 `json:"spread"`
	NEpochs            int     `json:"n_epochs"`
	LearningRate       float64 `json:"learning_rate"`
	NegativeSampleRate int     `json:"negative_sample_rate"`
	Metric             string  `json:"metric"`
	Init               string  `json:"init"`
	RandomState        int64   `json:"random_state"`
	NumWorkers         int     `json:"num_workers"`
}

type umapSnapshot struct {
	Params             umapParams  `json:"params"`
	Trustworthiness    float64     `json:"trustworthiness"`
	Embedding          [][]float64 `json:"embedding"`
	CondensedDistances []float64   `json:"condensed_distances"`
}

type umapSnapshotResult struct {
	snapshot umapSnapshot
	stderr   string
	err      error
}

func TestUMAP_CompareWithPythonBaseline(t *testing.T) {
	dataPath := filepath.Join("testdata", "tsne_dataset.csv")
	data := loadTSNECSV(t, dataPath)

	pythonResultCh := make(chan umapSnapshotResult, 1)
	go func() {
		snapshot, stderr, err := loadUMAPSnapshotViaUV()
		pythonResultCh <- umapSnapshotResult{
			snapshot: snapshot,
			stderr:   stderr,
			err:      err,
		}
	}()

	params := defaultUMAPParams()
	model, err := manifold.NewUMAP(
		params.NComponents,
		manifold.WithUMAPNNeighbors(params.NNeighbors),
		manifold.WithUMAPMinDist(params.MinDist),
		manifold.WithUMAPSpread(params.Spread),
		manifold.WithUMAPNEpochs(params.NEpochs),
		manifold.WithUMAPLearningRate(params.LearningRate),
		manifold.WithUMAPNegativeSampleRate(params.NegativeSampleRate),
		manifold.WithUMAPMetric(params.Metric),
		manifold.WithUMAPInit(manifold.UMAPInitMethod(params.Init)),
		manifold.WithUMAPRandomSeed(params.RandomState),
		manifold.WithUMAPNumWorkers(params.NumWorkers),
	)
	if err != nil {
		t.Fatalf("new umap failed: %v", err)
	}

	embedding, err := model.FitTransform(data)
	if err != nil {
		t.Fatalf("umap fit transform failed: %v", err)
	}

	pythonResult := <-pythonResultCh
	if pythonResult.err != nil {
		t.Fatalf(
			"run python umap snapshot via uv failed: %v\nstderr:\n%s",
			pythonResult.err,
			pythonResult.stderr,
		)
	}
	expected := pythonResult.snapshot

	goSnapshot := umapSnapshot{
		Params:             params,
		Trustworthiness:    numutil.Trustworthiness(data, embedding, 5),
		Embedding:          numutil.Clone2DFloat64(embedding),
		CondensedDistances: numutil.CondensedPairwiseDistances(embedding),
	}

	if len(goSnapshot.Embedding) != len(expected.Embedding) {
		t.Fatalf("embedding row mismatch: got=%d want=%d", len(goSnapshot.Embedding), len(expected.Embedding))
	}
	if len(goSnapshot.Embedding) == 0 || len(goSnapshot.Embedding[0]) != len(expected.Embedding[0]) {
		t.Fatalf("embedding column mismatch")
	}

	gotNormalized := numutil.NormalizeByMean(goSnapshot.CondensedDistances)
	wantNormalized := numutil.NormalizeByMean(expected.CondensedDistances)
	correlation := numutil.PearsonCorrelation(gotNormalized, wantNormalized)
	t.Logf("umap distance correlation: %.6f", correlation)
	if correlation < 0.60 {
		t.Fatalf("distance structure correlation too low: got=%.6f want>=%.6f", correlation, 0.60)
	}

	if diff := math.Abs(goSnapshot.Trustworthiness - expected.Trustworthiness); diff > 0.08 {
		t.Fatalf(
			"trustworthiness mismatch: got=%.6f want=%.6f diff=%.6f",
			goSnapshot.Trustworthiness,
			expected.Trustworthiness,
			diff,
		)
	}
	t.Logf(
		"umap trustworthiness go=%.6f python=%.6f diff=%.6f",
		goSnapshot.Trustworthiness,
		expected.Trustworthiness,
		math.Abs(goSnapshot.Trustworthiness-expected.Trustworthiness),
	)

	renderUMAPComparisonPlotViaUV(
		t,
		umapPlotRequest{
			CaseName:       "umap",
			PythonSnapshot: expected,
			GoSnapshot:     goSnapshot,
			OutputFile:     filepath.Join("testdata", "umap_compare.png"),
		},
	)
}

func defaultUMAPParams() umapParams {
	return umapParams{
		NComponents:        2,
		NNeighbors:         5,
		MinDist:            0.1,
		Spread:             1.0,
		NEpochs:            300,
		LearningRate:       1.0,
		NegativeSampleRate: 5,
		Metric:             "euclidean",
		Init:               "random",
		RandomState:        42,
		NumWorkers:         1,
	}
}

func loadUMAPSnapshotViaUV() (umapSnapshot, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"umap_test.py",
		"--snapshot-stdout",
	)
	cmd.Dir = "."

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return umapSnapshot{}, stderr.String(), err
	}

	var snapshot umapSnapshot
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &snapshot); err != nil {
		return umapSnapshot{}, stderr.String(), fmt.Errorf("decode python umap snapshot failed: %w", err)
	}
	return snapshot, stderr.String(), nil
}

type umapPlotRequest struct {
	CaseName       string       `json:"case_name"`
	PythonSnapshot umapSnapshot `json:"python_snapshot"`
	GoSnapshot     umapSnapshot `json:"go_snapshot"`
	OutputFile     string       `json:"output_file,omitempty"`
}

func renderUMAPComparisonPlotViaUV(t *testing.T, payload umapPlotRequest) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal umap plot payload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"umap_test.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render umap comparison plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python plot output: %s", output)
	}
}

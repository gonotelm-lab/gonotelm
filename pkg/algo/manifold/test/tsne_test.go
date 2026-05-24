package test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

const uvPythonPath = "../../.venv/bin/python"

type tsneParams struct {
	NComponents          int     `json:"n_components"`
	Perplexity           float64 `json:"perplexity"`
	EarlyExaggeration    float64 `json:"early_exaggeration"`
	LearningRate         string  `json:"learning_rate"`
	MaxIter              int     `json:"max_iter"`
	NIterWithoutProgress int     `json:"n_iter_without_progress"`
	MinGradNorm          float64 `json:"min_grad_norm"`
	Init                 string  `json:"init"`
	Method               string  `json:"method"`
	RandomState          int64   `json:"random_state"`
}

type tsneSklearnSnapshot struct {
	Params             tsneParams  `json:"params"`
	NIter              int         `json:"n_iter"`
	LearningRate       float64     `json:"learning_rate"`
	KLDivergence       float64     `json:"kl_divergence"`
	Trustworthiness    float64     `json:"trustworthiness"`
	Embedding          [][]float64 `json:"embedding"`
	CondensedDistances []float64   `json:"condensed_distances"`
}

func TestTSNE_CompareWithSklearnBaseline(t *testing.T) {
	dataPath := filepath.Join("testdata", "tsne_dataset.csv")
	data := loadTSNECSV(t, dataPath)

	pythonResultCh := make(chan tsneSnapshotResult, 1)
	go func() {
		snapshot, stderr, err := loadSklearnSnapshotViaUV()
		pythonResultCh <- tsneSnapshotResult{
			snapshot: snapshot,
			stderr:   stderr,
			err:      err,
		}
	}()

	params := defaultTSNEParams()
	model, err := manifold.NewTSNE(
		params.NComponents,
		manifold.WithPerplexity(params.Perplexity),
		manifold.WithEarlyExaggeration(params.EarlyExaggeration),
		manifold.WithAutoLearningRate(),
		manifold.WithMaxIter(params.MaxIter),
		manifold.WithNIterWithoutProgress(params.NIterWithoutProgress),
		manifold.WithMinGradNorm(params.MinGradNorm),
		manifold.WithInitMethod(manifold.InitMethod(params.Init)),
		manifold.WithRandomSeed(params.RandomState),
	)
	if err != nil {
		t.Fatalf("new tsne failed: %v", err)
	}

	embedding, err := model.FitTransform(data)
	if err != nil {
		t.Fatalf("fit transform failed: %v", err)
	}

	pythonResult := <-pythonResultCh
	if pythonResult.err != nil {
		t.Fatalf(
			"run sklearn snapshot via uv failed: %v\nstderr:\n%s",
			pythonResult.err,
			pythonResult.stderr,
		)
	}
	expected := pythonResult.snapshot

	if expected.Params.Method != "exact" {
		t.Fatalf("unsupported sklearn snapshot method=%q, expected exact", expected.Params.Method)
	}
	if expected.Params.LearningRate != "auto" {
		t.Fatalf("unsupported sklearn snapshot learning_rate=%q, expected auto", expected.Params.LearningRate)
	}

	goSnapshot := tsneSklearnSnapshot{
		Params:             params,
		NIter:              model.NIter(),
		LearningRate:       model.LearningRate(),
		KLDivergence:       model.KLDivergence(),
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

	if diff := math.Abs(goSnapshot.LearningRate - expected.LearningRate); diff > 1e-12 {
		t.Fatalf(
			"learning rate mismatch: got=%.12f want=%.12f diff=%.12f",
			goSnapshot.LearningRate,
			expected.LearningRate,
			diff,
		)
	}

	// t-SNE embedding may differ by global rotation/reflection/scale.
	// Compare condensed pairwise distances after mean normalization.
	gotNormalized := numutil.NormalizeByMean(goSnapshot.CondensedDistances)
	wantNormalized := numutil.NormalizeByMean(expected.CondensedDistances)
	correlation := numutil.PearsonCorrelation(gotNormalized, wantNormalized)
	if correlation < 0.85 {
		t.Fatalf("distance structure correlation too low: got=%.6f want>=%.6f", correlation, 0.85)
	}

	// Compare local neighborhood preservation quality.
	if diff := math.Abs(goSnapshot.Trustworthiness - expected.Trustworthiness); diff > 0.03 {
		t.Fatalf(
			"trustworthiness mismatch: got=%.6f want=%.6f diff=%.6f",
			goSnapshot.Trustworthiness,
			expected.Trustworthiness,
			diff,
		)
	}

	// Non-convex optimization can converge to different minima while preserving local neighborhoods.
	if diff := math.Abs(goSnapshot.KLDivergence - expected.KLDivergence); diff > 0.25 {
		t.Fatalf(
			"kl divergence mismatch: got=%.12f want=%.12f diff=%.12f",
			goSnapshot.KLDivergence,
			expected.KLDivergence,
			diff,
		)
	}

	if gotIter, wantIter := goSnapshot.NIter, expected.NIter; absInt(gotIter-wantIter) > 30 {
		t.Fatalf("iteration mismatch: got=%d want=%d", gotIter, wantIter)
	}

	renderComparisonPlotViaUV(
		t,
		tsnePlotRequest{
			CaseName:        "tsne",
			SklearnSnapshot: expected,
			GoSnapshot:      goSnapshot,
			OutputFile:      filepath.Join("testdata", "tsne_compare.png"),
		},
	)
}

func loadTSNECSV(t *testing.T, path string) [][]float64 {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv failed: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("csv is empty: %s", path)
	}

	result := make([][]float64, len(records))
	for i, record := range records {
		result[i] = make([]float64, len(record))
		for j, raw := range record {
			value, parseErr := strconv.ParseFloat(raw, 64)
			if parseErr != nil {
				t.Fatalf("parse csv[%d][%d]=%q failed: %v", i, j, raw, parseErr)
			}
			result[i][j] = value
		}
	}
	return result
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func defaultTSNEParams() tsneParams {
	return tsneParams{
		NComponents:          2,
		Perplexity:           5.0,
		EarlyExaggeration:    12.0,
		LearningRate:         "auto",
		MaxIter:              600,
		NIterWithoutProgress: 300,
		MinGradNorm:          1e-7,
		Init:                 "pca",
		Method:               "exact",
		RandomState:          42,
	}
}

type tsneSnapshotResult struct {
	snapshot tsneSklearnSnapshot
	stderr   string
	err      error
}

func loadSklearnSnapshotViaUV() (tsneSklearnSnapshot, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"tsne_test.py",
		"--snapshot-stdout",
	)
	cmd.Dir = "."

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return tsneSklearnSnapshot{}, stderr.String(), err
	}

	var snapshot tsneSklearnSnapshot
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &snapshot); err != nil {
		return tsneSklearnSnapshot{}, stderr.String(), fmt.Errorf("decode sklearn snapshot failed: %w", err)
	}
	return snapshot, stderr.String(), nil
}

type tsnePlotRequest struct {
	CaseName        string              `json:"case_name"`
	SklearnSnapshot tsneSklearnSnapshot `json:"sklearn_snapshot"`
	GoSnapshot      tsneSklearnSnapshot `json:"go_snapshot"`
	OutputFile      string              `json:"output_file,omitempty"`
}

func renderComparisonPlotViaUV(t *testing.T, payload tsnePlotRequest) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal tsne plot payload failed: %v", err)
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
		"tsne_test.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render tsne comparison plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python plot output: %s", output)
	}
}

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

	"github.com/gonotelm-lab/gonotelm/pkg/algo/decomposition"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

const uvPythonPath = "../../.venv/bin/python"

type sklearnSnapshot struct {
	NComponents            int         `json:"n_components"`
	Mean                   []float64   `json:"mean"`
	Components             [][]float64 `json:"components"`
	ExplainedVariance      []float64   `json:"explained_variance"`
	ExplainedVarianceRatio []float64   `json:"explained_variance_ratio"`
	SingularValues         []float64   `json:"singular_values"`
	Transformed            [][]float64 `json:"transformed"`
}

func TestPCA_MatchesSklearnBaseline(t *testing.T) {
	dataPath := filepath.Join("testdata", "pca_dataset.csv")
	data := loadCSV(t, dataPath)

	pythonResultCh := make(chan pcaSnapshotResult, 1)
	go func() {
		snapshot, stderr, err := loadSklearnSnapshotViaUV()
		pythonResultCh <- pcaSnapshotResult{
			snapshot: snapshot,
			stderr:   stderr,
			err:      err,
		}
	}()

	model := decomposition.NewPCA(2)
	transformed, err := model.FitTransform(data)
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

	gotComponents := numutil.Clone2DFloat64(model.Components())
	gotTransformed := numutil.Clone2DFloat64(transformed)
	alignComponentSigns(gotComponents, gotTransformed, expected.Components)

	goSnapshot := sklearnSnapshot{
		NComponents:            model.NComponents(),
		Mean:                   model.Mean(),
		Components:             gotComponents,
		ExplainedVariance:      model.ExplainedVariance(),
		ExplainedVarianceRatio: model.ExplainedVarianceRatio(),
		SingularValues:         model.SingularValues(),
		Transformed:            gotTransformed,
	}

	assertCloseVector(t, "mean", goSnapshot.Mean, expected.Mean, 1e-12)
	assertCloseMatrix(t, "components", goSnapshot.Components, expected.Components, 1e-9)
	assertCloseVector(t, "explained_variance", goSnapshot.ExplainedVariance, expected.ExplainedVariance, 1e-10)
	assertCloseVector(
		t,
		"explained_variance_ratio",
		goSnapshot.ExplainedVarianceRatio,
		expected.ExplainedVarianceRatio,
		1e-10,
	)
	assertCloseVector(t, "singular_values", goSnapshot.SingularValues, expected.SingularValues, 1e-10)
	assertCloseMatrix(t, "transformed", goSnapshot.Transformed, expected.Transformed, 1e-8)

	renderComparisonPlotViaUV(
		t,
		pcaPlotRequest{
			CaseName:        "pca",
			Vectors:         data,
			SklearnSnapshot: expected,
			GoSnapshot:      goSnapshot,
			OutputFile:      filepath.Join("testdata", "pca_compare.png"),
		},
	)
}

func loadCSV(t *testing.T, path string) [][]float64 {
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

func alignComponentSigns(components, transformed, expectedComponents [][]float64) {
	if len(components) == 0 || len(components) != len(expectedComponents) {
		return
	}

	for componentIdx := range components {
		dot := 0.0
		for featureIdx := range components[componentIdx] {
			dot += components[componentIdx][featureIdx] * expectedComponents[componentIdx][featureIdx]
		}
		if dot >= 0 {
			continue
		}
		for featureIdx := range components[componentIdx] {
			components[componentIdx][featureIdx] = -components[componentIdx][featureIdx]
		}
		for sampleIdx := range transformed {
			transformed[sampleIdx][componentIdx] = -transformed[sampleIdx][componentIdx]
		}
	}
}

func assertCloseMatrix(t *testing.T, name string, got, want [][]float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s row count mismatch: got=%d want=%d", name, len(got), len(want))
	}
	for i := range got {
		assertCloseVector(t, fmt.Sprintf("%s[%d]", name, i), got[i], want[i], tol)
	}
}

func assertCloseVector(t *testing.T, name string, got, want []float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got=%d want=%d", name, len(got), len(want))
	}
	for idx := range got {
		diff := math.Abs(got[idx] - want[idx])
		if diff > tol {
			t.Fatalf(
				"%s mismatch at index %d: got=%.15f want=%.15f diff=%.15f tol=%.15f",
				name,
				idx,
				got[idx],
				want[idx],
				diff,
				tol,
			)
		}
	}
}

type pcaSnapshotResult struct {
	snapshot sklearnSnapshot
	stderr   string
	err      error
}

type pcaPlotRequest struct {
	CaseName        string          `json:"case_name"`
	Vectors         [][]float64     `json:"vectors"`
	SklearnSnapshot sklearnSnapshot `json:"sklearn_snapshot"`
	GoSnapshot      sklearnSnapshot `json:"go_snapshot"`
	OutputFile      string          `json:"output_file,omitempty"`
}

func loadSklearnSnapshotViaUV() (sklearnSnapshot, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"uv",
		"run",
		"--python",
		uvPythonPath,
		"python",
		"pca_test.py",
		"--snapshot-stdout",
	)
	cmd.Dir = "."

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return sklearnSnapshot{}, stderr.String(), err
	}

	var snapshot sklearnSnapshot
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &snapshot); err != nil {
		return sklearnSnapshot{}, stderr.String(), fmt.Errorf("decode sklearn snapshot failed: %w", err)
	}
	return snapshot, stderr.String(), nil
}

func renderComparisonPlotViaUV(t *testing.T, payload pcaPlotRequest) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pca plot payload failed: %v", err)
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
		"pca_test.py",
		"--plot-stdin",
	)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render pca comparison plot failed: %v\nstderr:\n%s", err, stderr.String())
	}
	if output := strings.TrimSpace(stdout.String()); output != "" {
		t.Logf("python plot output: %s", output)
	}
}

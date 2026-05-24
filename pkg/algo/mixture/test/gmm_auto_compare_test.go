package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/numutil"
)

func TestGaussianMixture_AutoSelectCompareWithSklearn(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve current file path failed")
	}
	testdataDir := filepath.Join(filepath.Dir(thisFile), "testdata")
	caseFilePath := filepath.Join(testdataDir, casesFileName)
	cases := loadCases(t, caseFilePath)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			data := loadDatasetForCase(t, testdataDir, tc)

			pythonResultCh := make(chan sklearnSnapshotResult, 1)
			go func() {
				snapshot, stderr, err := loadSklearnAutoSnapshotViaUV(tc.Name)
				pythonResultCh <- sklearnSnapshotResult{
					snapshot: snapshot,
					stderr:   stderr,
					err:      err,
				}
			}()

			model, evaluation, _, err := mixture.AutoSelectGaussianMixtureDefault(
				data.Vectors,
				mixture.WithTolerance(tc.Tolerance),
				mixture.WithRegularization(tc.Regularization),
				mixture.WithMaxIterations(tc.MaxIterations),
				mixture.WithNInit(max(tc.RequestedNInit, 1)),
				mixture.WithInitParams(parseInitParams(tc.InitParams)),
				mixture.WithRandomSeed(tc.Seed),
			)
			if err != nil {
				t.Fatalf("auto select fit_evaluate failed: %v", err)
			}

			goSnapshot := gmmSnapshot{
				Name:           tc.Name,
				DatasetFile:    tc.SourceFile,
				CovarianceType: defaultIfEmpty(tc.CovarianceType, "full"),
				ClusterCount:   len(evaluation.Weights),
				Labels:         evaluation.Labels,
				Weights:        evaluation.Weights,
				Means:          evaluation.Means,
				LogLikelihood:  evaluation.LogLikelihood,
				BIC:            evaluation.BIC,
				Iterations:     evaluation.Iterations,
				Converged:      evaluation.Converged,
			}
			if len(data.TrueLabels) > 0 {
				ariTrue := numutil.AdjustedRandIndex(goSnapshot.Labels, data.TrueLabels)
				goSnapshot.ARIVsTrueLabels = &ariTrue
			}

			pythonResult := <-pythonResultCh
			if pythonResult.err != nil {
				t.Fatalf(
					"run sklearn auto snapshot via uv failed: %v\nstderr:\n%s",
					pythonResult.err,
					pythonResult.stderr,
				)
			}
			sklearnSnapshot := pythonResult.snapshot

			canonicalizeSnapshot(&sklearnSnapshot)
			canonicalizeSnapshot(&goSnapshot)

			ariGoVsSklearn := numutil.AdjustedRandIndex(goSnapshot.Labels, sklearnSnapshot.Labels)

			renderComparisonPlotViaUV(
				t,
				gmmComparePlotRequest{
					CaseName:        fmt.Sprintf("%s_auto", tc.Name),
					Vectors:         data.Vectors,
					SklearnSnapshot: sklearnSnapshot,
					GoSnapshot:      goSnapshot,
					OutputFile: filepath.Join(
						testdataDir,
						fmt.Sprintf("gmm_auto_compare_%s.png", tc.Name),
					),
				},
			)

			if ariGoVsSklearn < 0.2 {
				t.Logf("warning: auto-select ari(go, sklearn) is low for %s: %.6f", tc.Name, ariGoVsSklearn)
			}
			if diff := math.Abs(float64(goSnapshot.ClusterCount - sklearnSnapshot.ClusterCount)); diff > 2 {
				t.Logf(
					"warning: selected cluster_count diverges for %s: go=%d sklearn=%d",
					tc.Name,
					goSnapshot.ClusterCount,
					sklearnSnapshot.ClusterCount,
				)
			}
			if tc.Name == "fake_1" && goSnapshot.ClusterCount != 3 {
				t.Fatalf("fake_1 expected selected cluster_count=3, got=%d", goSnapshot.ClusterCount)
			}

			t.Logf(
				"auto-select %s: go_k=%d sklearn_k=%d ari(go,sk)=%.4f",
				tc.Name,
				goSnapshot.ClusterCount,
				sklearnSnapshot.ClusterCount,
				ariGoVsSklearn,
			)

			_ = model
		})
	}
}

func loadSklearnAutoSnapshotViaUV(caseName string) (gmmSnapshot, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
		"--snapshot-auto-stdout",
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
		return gmmSnapshot{}, stderr.String(), fmt.Errorf("decode sklearn auto snapshot json failed: %w", err)
	}
	return snapshot, stderr.String(), nil
}

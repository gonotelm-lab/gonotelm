package algo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	algo "github.com/gonotelm-lab/gonotelm/pkg/algo"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

func TestPCAGMMCompareOnEmbeddings(t *testing.T) {
	const (
		targetDim      = 64
		clusterCount   = 8
		maxIterations  = 120
		tolerance      = 1e-6
		regularization = 1e-6
		seed           = 7
	)

	pkgDir := pcaGMMTestDir(t)
	datasetPath := filepath.Join(pkgDir, "embeddings.jsonl")
	outputFigurePath := filepath.Join(pkgDir, "pca_gmm_compare_embed.png")

	vectors, err := loadEmbeddingsJSONL(datasetPath)
	if err != nil {
		t.Fatalf("load embeddings failed: %v", err)
	}
	reducedVectors, err := reduceVectorsWithPCA(vectors, targetDim)
	if err != nil {
		t.Fatalf("go PCA reduce failed: %v", err)
	}

	goResult, err := algo.GMMCluster(reducedVectors, clusterCount, &algo.GMMOptions{
		MaxIterations:  maxIterations,
		Tolerance:      tolerance,
		Regularization: regularization,
		Seed:           seed,
	})
	if err != nil {
		t.Fatalf("go GMM cluster failed: %v", err)
	}

	pyResult, pyOutput, err := runPythonPCAGMMCompare(pkgDir, pcaGMMPythonInput{
		Vectors:          vectors,
		GoLabels:         goResult.Labels,
		GoMeans:          goResult.Means,
		TargetDim:        targetDim,
		ClusterCount:     clusterCount,
		MaxIterations:    maxIterations,
		Tolerance:        tolerance,
		Regularization:   regularization,
		Seed:             seed,
		OutputFigurePath: outputFigurePath,
	})
	if err != nil {
		t.Fatalf("python PCA+GMM compare failed: %v\npython output:\n%s", err, pyOutput)
	}

	if pyResult.ARI < 0.50 {
		t.Fatalf("ARI too low, got=%.4f want>=0.50", pyResult.ARI)
	}
	if pyResult.NMI < 0.70 {
		t.Fatalf("NMI too low, got=%.4f want>=0.70", pyResult.NMI)
	}

	t.Logf(
		"PCA64+GMM compare done: ARI=%.4f NMI=%.4f go_sil=%.4f py_sil=%.4f figure=%s",
		pyResult.ARI,
		pyResult.NMI,
		pyResult.GoSilhouette,
		pyResult.PySilhouette,
		outputFigurePath,
	)
}

type pcaGMMPythonInput struct {
	Vectors          [][]float64 `json:"vectors"`
	GoLabels         []int       `json:"go_labels"`
	GoMeans          [][]float64 `json:"go_means"`
	TargetDim        int         `json:"target_dim"`
	ClusterCount     int         `json:"cluster_count"`
	MaxIterations    int         `json:"max_iterations"`
	Tolerance        float64     `json:"tolerance"`
	Regularization   float64     `json:"regularization"`
	Seed             int         `json:"seed"`
	OutputFigurePath string      `json:"output_figure_path"`
}

type pcaGMMPythonResult struct {
	ARI          float64 `json:"ari"`
	NMI          float64 `json:"nmi"`
	GoSilhouette float64 `json:"go_silhouette"`
	PySilhouette float64 `json:"py_silhouette"`
	PyIterations int     `json:"py_iterations"`
}

func runPythonPCAGMMCompare(pkgDir string, input pcaGMMPythonInput) (*pcaGMMPythonResult, string, error) {
	inputFile, err := os.CreateTemp("", "pca_gmm_compare_*.json")
	if err != nil {
		return nil, "", fmt.Errorf("create temp input file failed: %w", err)
	}
	inputPath := inputFile.Name()
	defer os.Remove(inputPath)
	defer inputFile.Close()

	data, err := json.Marshal(input)
	if err != nil {
		return nil, "", fmt.Errorf("marshal python input failed: %w", err)
	}
	if _, err = inputFile.Write(data); err != nil {
		return nil, "", fmt.Errorf("write python input failed: %w", err)
	}

	mplDir, err := os.MkdirTemp("", "mplconfig-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp mpl dir failed: %w", err)
	}
	defer os.RemoveAll(mplDir)

	scriptPath := filepath.Join(pkgDir, "pca_gmm.py")
	cmd := exec.Command("uv", "run", "python", scriptPath, inputPath)
	cmd.Dir = pkgDir
	cmd.Env = append(os.Environ(), "MPLCONFIGDIR="+mplDir)
	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)
	if err != nil {
		return nil, output, fmt.Errorf("run python command failed: %w", err)
	}

	var jsonLine string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "RESULT_JSON:") {
			jsonLine = strings.TrimPrefix(line, "RESULT_JSON:")
			break
		}
	}
	if jsonLine == "" {
		return nil, output, fmt.Errorf("cannot find RESULT_JSON line in python output")
	}

	var result pcaGMMPythonResult
	if err = json.Unmarshal([]byte(jsonLine), &result); err != nil {
		return nil, output, fmt.Errorf("parse python result json failed: %w", err)
	}
	return &result, output, nil
}

func reduceVectorsWithPCA(vectors [][]float64, targetDim int) ([][]float64, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("vectors are empty")
	}
	if len(vectors[0]) == 0 {
		return nil, fmt.Errorf("vector dimension must be greater than 0")
	}

	sampleCount := len(vectors)
	featureDim := len(vectors[0])
	if targetDim <= 0 || targetDim > featureDim {
		return nil, fmt.Errorf("invalid target dimension %d for feature dim %d", targetDim, featureDim)
	}

	data := make([]float64, sampleCount*featureDim)
	for i := range vectors {
		if len(vectors[i]) != featureDim {
			return nil, fmt.Errorf("inconsistent vector dim at row=%d", i)
		}
		copy(data[i*featureDim:(i+1)*featureDim], vectors[i])
	}
	samples := mat.NewDense(sampleCount, featureDim, data)

	var pc stat.PC
	if ok := pc.PrincipalComponents(samples, nil); !ok {
		return nil, fmt.Errorf("principal components analysis failed")
	}

	means := make([]float64, featureDim)
	for c := 0; c < featureDim; c++ {
		col := mat.Col(nil, c, samples)
		means[c] = stat.Mean(col, nil)
	}
	centeredData := make([]float64, sampleCount*featureDim)
	for r := 0; r < sampleCount; r++ {
		for c := 0; c < featureDim; c++ {
			centeredData[r*featureDim+c] = samples.At(r, c) - means[c]
		}
	}
	centered := mat.NewDense(sampleCount, featureDim, centeredData)

	var vectorsMat mat.Dense
	pc.VectorsTo(&vectorsMat)
	components := vectorsMat.Slice(0, featureDim, 0, targetDim)

	var reduced mat.Dense
	reduced.Mul(centered, components)

	out := make([][]float64, sampleCount)
	for r := 0; r < sampleCount; r++ {
		out[r] = mat.Row(nil, r, &reduced)
	}
	return out, nil
}

func pcaGMMTestDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve current test file path")
	}
	return filepath.Dir(file)
}

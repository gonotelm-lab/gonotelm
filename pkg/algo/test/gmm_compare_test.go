package algo_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	algo "github.com/gonotelm-lab/gonotelm/pkg/algo"
)

const (
	gmmCompareSeed           int64   = 7
	gmmCompareMaxIterations          = 120
	gmmCompareTolerance      float64 = 1e-6
	gmmCompareRegularization float64 = 1e-6

	gmmCompareFakeDataSeed int64 = 42

	gmmCompareFakeOutputPath  = "gmm_compare_fake.json"
	gmmCompareEmbedOutputPath = "gmm_compare_embed.json"
	gmmCompareEmbedDataset    = "embeddings.jsonl"
)

type gmmComparePayload struct {
	Name           string             `json:"name"`
	DatasetPath    string             `json:"dataset_path,omitempty"`
	Vectors        [][]float64        `json:"vectors"`
	TrueLabels     []int              `json:"true_labels,omitempty"`
	ClusterCount   int                `json:"cluster_count"`
	MaxIterations  int                `json:"max_iterations"`
	Tolerance      float64            `json:"tolerance"`
	Regularization float64            `json:"regularization"`
	Seed           int64              `json:"seed"`
	GoResult       gmmCompareGoResult `json:"go_result"`
}

type gmmCompareGoResult struct {
	Labels        []int       `json:"labels"`
	Weights       []float64   `json:"weights"`
	Means         [][]float64 `json:"means"`
	ClusterCount  int         `json:"cluster_count"`
	LogLikelihood float64     `json:"log_likelihood"`
	BIC           float64     `json:"bic"`
	Iterations    int         `json:"iterations"`
}

func TestGMMCompareExport(t *testing.T) {
	if os.Getenv("GMM_COMPARE_EXPORT") != "1" {
		t.Skip("set GMM_COMPARE_EXPORT=1 to export compare data")
	}

	t.Run("fake", func(t *testing.T) {
		vectors, labels := generateFakeCompareData(gmmCompareFakeDataSeed)
		payload, err := buildComparePayload(
			"fake",
			"",
			vectors,
			labels,
			3,
		)
		if err != nil {
			t.Fatalf("build fake compare payload failed: %v", err)
		}
		if err = writeComparePayloadJSON(gmmCompareFakeOutputPath, payload); err != nil {
			t.Fatalf("write fake compare payload failed: %v", err)
		}
		t.Logf("wrote fake payload: %s", gmmCompareFakeOutputPath)
	})

	t.Run("embed", func(t *testing.T) {
		vectors, err := loadEmbeddingsJSONL(gmmCompareEmbedDataset)
		if err != nil {
			t.Fatalf("load embeddings jsonl failed: %v", err)
		}
		payload, err := buildComparePayload(
			"embed",
			gmmCompareEmbedDataset,
			vectors,
			nil,
			8,
		)
		if err != nil {
			t.Fatalf("build embed compare payload failed: %v", err)
		}
		if err = writeComparePayloadJSON(gmmCompareEmbedOutputPath, payload); err != nil {
			t.Fatalf("write embed compare payload failed: %v", err)
		}
		t.Logf("wrote embed payload: %s", gmmCompareEmbedOutputPath)
	})
}

func buildComparePayload(
	name string,
	datasetPath string,
	vectors [][]float64,
	trueLabels []int,
	clusterCount int,
) (*gmmComparePayload, error) {
	result, err := algo.GMMCluster(vectors, clusterCount, &algo.GMMOptions{
		MaxIterations:  gmmCompareMaxIterations,
		Tolerance:      gmmCompareTolerance,
		Regularization: gmmCompareRegularization,
		Seed:           gmmCompareSeed,
	})
	if err != nil {
		return nil, fmt.Errorf("run gmm cluster failed: %w", err)
	}

	return &gmmComparePayload{
		Name:           name,
		DatasetPath:    datasetPath,
		Vectors:        vectors,
		TrueLabels:     append([]int(nil), trueLabels...),
		ClusterCount:   clusterCount,
		MaxIterations:  gmmCompareMaxIterations,
		Tolerance:      gmmCompareTolerance,
		Regularization: gmmCompareRegularization,
		Seed:           gmmCompareSeed,
		GoResult:       toCompareGoResult(result),
	}, nil
}

func toCompareGoResult(result *algo.GMMResult) gmmCompareGoResult {
	return gmmCompareGoResult{
		Labels:        append([]int(nil), result.Labels...),
		Weights:       append([]float64(nil), result.Weights...),
		Means:         clone2DSlice(result.Means),
		ClusterCount:  result.ClusterCount,
		LogLikelihood: result.LogLikelihood,
		BIC:           result.BIC,
		Iterations:    result.Iterations,
	}
}

func writeComparePayloadJSON(path string, payload *gmmComparePayload) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir output dir failed: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(payload); err != nil {
		return fmt.Errorf("encode payload json failed: %w", err)
	}
	return nil
}

func loadEmbeddingsJSONL(path string) ([][]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open embeddings jsonl failed: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		vectors [][]float64
		dim     = -1
		lineNum = 0
	)
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var row []float64
		if err = json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("line %d json unmarshal failed: %w", lineNum, err)
		}
		if len(row) == 0 {
			return nil, fmt.Errorf("line %d has empty vector", lineNum)
		}
		if dim == -1 {
			dim = len(row)
		} else if len(row) != dim {
			return nil, fmt.Errorf("line %d dimension mismatch, got=%d want=%d", lineNum, len(row), dim)
		}
		for i, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("line %d invalid value at idx=%d", lineNum, i)
			}
		}
		vectors = append(vectors, row)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan jsonl failed: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no vectors loaded from %s", path)
	}
	return vectors, nil
}

func generateFakeCompareData(seed int64) ([][]float64, []int) {
	rng := rand.New(rand.NewSource(seed))
	means := [][]float64{
		{-4.5, -1.2},
		{0.8, 4.0},
		{4.8, -0.5},
	}
	covariances := [][][]float64{
		{{1.1, 0.25}, {0.25, 0.7}},
		{{0.8, -0.3}, {-0.3, 0.9}},
		{{0.9, 0.2}, {0.2, 0.6}},
	}
	counts := []int{260, 250, 240}

	total := 0
	for _, c := range counts {
		total += c
	}
	vectors := make([][]float64, 0, total)
	labels := make([]int, 0, total)

	for k := range means {
		a := covariances[k][0][0]
		b := covariances[k][0][1]
		c := covariances[k][1][1]
		l11 := math.Sqrt(a)
		l21 := b / l11
		l22 := math.Sqrt(c - l21*l21)

		for i := 0; i < counts[k]; i++ {
			z1 := rng.NormFloat64()
			z2 := rng.NormFloat64()
			x := means[k][0] + l11*z1
			y := means[k][1] + l21*z1 + l22*z2
			vectors = append(vectors, []float64{x, y})
			labels = append(labels, k)
		}
	}

	return vectors, labels
}

func clone2DSlice(in [][]float64) [][]float64 {
	out := make([][]float64, len(in))
	for i := range in {
		out[i] = append([]float64(nil), in[i]...)
	}
	return out
}

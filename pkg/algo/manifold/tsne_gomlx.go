package manifold

import (
	"fmt"
	"math"
	"slices"

	"github.com/gomlx/gomlx/backends"
	_ "github.com/gomlx/gomlx/backends/default"
	"github.com/gomlx/gomlx/pkg/core/dtypes"
	"github.com/gomlx/gomlx/pkg/core/shapes"
)

type goMLXKLDivergenceKernel struct {
	backend    backends.Backend
	executable backends.Executable

	paramsShape shapes.Shape
	paramsLen   int

	klFlat   []float64
	gradFlat []float64
}

func newGoMLXKLDivergenceKernel(
	jointProbabilities []float64,
	degreesOfFreedom int,
	nSamples int,
	nComponents int,
	backendConfig string,
) (*goMLXKLDivergenceKernel, error) {
	backend, err := backends.NewWithConfig(backendConfig)
	if err != nil {
		return nil, fmt.Errorf("init gomlx backend failed: %w", err)
	}

	cleanupBackend := true
	defer func() {
		if cleanupBackend {
			backend.Finalize()
		}
	}()

	builder := backend.Builder("tsne_kl_gradient")
	fn := builder.Main()

	paramsShape := shapes.Make(dtypes.Float64, nSamples, nComponents)
	params, err := fn.Parameter("params", paramsShape, nil)
	if err != nil {
		return nil, fmt.Errorf("define params failed: %w", err)
	}

	shapeNN := shapes.Make(dtypes.Float64, nSamples, nSamples)
	jointConst, err := fn.Constant(slices.Clone(jointProbabilities), nSamples, nSamples)
	if err != nil {
		return nil, fmt.Errorf("define joint probabilities constant failed: %w", err)
	}

	offDiagonalMaskData := make([]float64, nSamples*nSamples)
	for i := 0; i < nSamples; i++ {
		rowOffset := i * nSamples
		for j := 0; j < nSamples; j++ {
			if i != j {
				offDiagonalMaskData[rowOffset+j] = 1
			}
		}
	}
	offDiagonalMask, err := fn.Constant(offDiagonalMaskData, nSamples, nSamples)
	if err != nil {
		return nil, fmt.Errorf("define off-diagonal mask constant failed: %w", err)
	}

	zeroScalar, err := fn.Constant([]float64{0})
	if err != nil {
		return nil, fmt.Errorf("define zero constant failed: %w", err)
	}
	oneScalar, err := fn.Constant([]float64{1})
	if err != nil {
		return nil, fmt.Errorf("define one constant failed: %w", err)
	}
	twoScalar, err := fn.Constant([]float64{2})
	if err != nil {
		return nil, fmt.Errorf("define two constant failed: %w", err)
	}
	epsilonScalar, err := fn.Constant([]float64{machineEpsilon})
	if err != nil {
		return nil, fmt.Errorf("define epsilon constant failed: %w", err)
	}

	dofFloat64 := float64(degreesOfFreedom)
	dofScalar, err := fn.Constant([]float64{dofFloat64})
	if err != nil {
		return nil, fmt.Errorf("define dof constant failed: %w", err)
	}
	power := -((dofFloat64 + 1.0) / 2.0)
	powerScalar, err := fn.Constant([]float64{power})
	if err != nil {
		return nil, fmt.Errorf("define power constant failed: %w", err)
	}
	gradientScale := 2.0 * (dofFloat64 + 1.0) / dofFloat64
	gradientScaleScalar, err := fn.Constant([]float64{gradientScale})
	if err != nil {
		return nil, fmt.Errorf("define gradient scale constant failed: %w", err)
	}

	paramsSquared, err := fn.Mul(params, params)
	if err != nil {
		return nil, fmt.Errorf("params squared failed: %w", err)
	}
	rowNorms, err := fn.ReduceSum(paramsSquared, 1)
	if err != nil {
		return nil, fmt.Errorf("row norm reduction failed: %w", err)
	}
	rowNormsI, err := fn.BroadcastInDim(rowNorms, shapeNN, []int{0})
	if err != nil {
		return nil, fmt.Errorf("row norm broadcast (axis=0) failed: %w", err)
	}
	rowNormsJ, err := fn.BroadcastInDim(rowNorms, shapeNN, []int{1})
	if err != nil {
		return nil, fmt.Errorf("row norm broadcast (axis=1) failed: %w", err)
	}

	paramsTransposed, err := fn.Transpose(params, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("transpose params failed: %w", err)
	}
	gramMatrix, err := fn.DotGeneral(
		params,
		[]int{1},
		nil,
		paramsTransposed,
		[]int{0},
		nil,
		backends.DotGeneralConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("gram matrix computation failed: %w", err)
	}
	twoGramMatrix, err := fn.Mul(twoScalar, gramMatrix)
	if err != nil {
		return nil, fmt.Errorf("scale gram matrix failed: %w", err)
	}
	distancesRaw, err := fn.Add(rowNormsI, rowNormsJ)
	if err != nil {
		return nil, fmt.Errorf("distance sum failed: %w", err)
	}
	distancesRaw, err = fn.Sub(distancesRaw, twoGramMatrix)
	if err != nil {
		return nil, fmt.Errorf("distance subtraction failed: %w", err)
	}
	distances, err := fn.Max(distancesRaw, zeroScalar)
	if err != nil {
		return nil, fmt.Errorf("distance clamp failed: %w", err)
	}

	distancesOverDof, err := fn.Div(distances, dofScalar)
	if err != nil {
		return nil, fmt.Errorf("distance scaling failed: %w", err)
	}
	numBase, err := fn.Add(oneScalar, distancesOverDof)
	if err != nil {
		return nil, fmt.Errorf("num base creation failed: %w", err)
	}
	numUnmasked, err := fn.Pow(numBase, powerScalar)
	if err != nil {
		return nil, fmt.Errorf("num power operation failed: %w", err)
	}
	num, err := fn.Mul(numUnmasked, offDiagonalMask)
	if err != nil {
		return nil, fmt.Errorf("num off-diagonal masking failed: %w", err)
	}

	sumNum, err := fn.ReduceSum(num)
	if err != nil {
		return nil, fmt.Errorf("sum(num) failed: %w", err)
	}
	sumNumSafe, err := fn.Max(sumNum, epsilonScalar)
	if err != nil {
		return nil, fmt.Errorf("sum(num) clamp failed: %w", err)
	}
	qRaw, err := fn.Div(num, sumNumSafe)
	if err != nil {
		return nil, fmt.Errorf("q normalization failed: %w", err)
	}
	q, err := fn.Max(qRaw, epsilonScalar)
	if err != nil {
		return nil, fmt.Errorf("q clamp failed: %w", err)
	}
	qSafe, err := fn.Max(q, epsilonScalar)
	if err != nil {
		return nil, fmt.Errorf("q safe clamp failed: %w", err)
	}

	jointSafe, err := fn.Max(jointConst, epsilonScalar)
	if err != nil {
		return nil, fmt.Errorf("joint probability clamp failed: %w", err)
	}
	ratio, err := fn.Div(jointSafe, qSafe)
	if err != nil {
		return nil, fmt.Errorf("kl ratio computation failed: %w", err)
	}
	logRatio, err := fn.Log(ratio)
	if err != nil {
		return nil, fmt.Errorf("kl log ratio computation failed: %w", err)
	}
	klTerms, err := fn.Mul(jointConst, logRatio)
	if err != nil {
		return nil, fmt.Errorf("kl weighted term computation failed: %w", err)
	}
	kl, err := fn.ReduceSum(klTerms)
	if err != nil {
		return nil, fmt.Errorf("kl reduction failed: %w", err)
	}

	pMinusQ, err := fn.Sub(jointConst, q)
	if err != nil {
		return nil, fmt.Errorf("p-q computation failed: %w", err)
	}
	weightedAttractionRepulsion, err := fn.Mul(pMinusQ, num)
	if err != nil {
		return nil, fmt.Errorf("(p-q)*num computation failed: %w", err)
	}
	rowSums, err := fn.ReduceSum(weightedAttractionRepulsion, 1)
	if err != nil {
		return nil, fmt.Errorf("row sum reduction failed: %w", err)
	}
	rowSumsMatrix, err := fn.BroadcastInDim(rowSums, shapeNN, []int{0})
	if err != nil {
		return nil, fmt.Errorf("row sum broadcast failed: %w", err)
	}
	laplacianLikeMatrix, err := fn.Sub(rowSumsMatrix, weightedAttractionRepulsion)
	if err != nil {
		return nil, fmt.Errorf("laplacian-like matrix computation failed: %w", err)
	}
	gradientRaw, err := fn.DotGeneral(
		laplacianLikeMatrix,
		[]int{1},
		nil,
		params,
		[]int{0},
		nil,
		backends.DotGeneralConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("gradient matrix multiplication failed: %w", err)
	}
	gradient, err := fn.Mul(gradientScaleScalar, gradientRaw)
	if err != nil {
		return nil, fmt.Errorf("gradient scaling failed: %w", err)
	}

	if err := fn.Return([]backends.Value{kl, gradient}, nil); err != nil {
		return nil, fmt.Errorf("set executable outputs failed: %w", err)
	}
	executable, err := builder.Compile()
	if err != nil {
		return nil, fmt.Errorf("compile gomlx executable failed: %w", err)
	}

	cleanupBackend = false
	return &goMLXKLDivergenceKernel{
		backend:     backend,
		executable:  executable,
		paramsShape: paramsShape,
		paramsLen:   nSamples * nComponents,
		klFlat:      make([]float64, 1),
		gradFlat:    make([]float64, nSamples*nComponents),
	}, nil
}

func (k *goMLXKLDivergenceKernel) Evaluate(params []float64) (float64, []float64, error) {
	if len(params) != k.paramsLen {
		return 0, nil, fmt.Errorf("params length mismatch: got=%d want=%d", len(params), k.paramsLen)
	}

	paramsBuffer, err := k.backend.BufferFromFlatData(0, params, k.paramsShape)
	if err != nil {
		return 0, nil, fmt.Errorf("upload params buffer failed: %w", err)
	}
	defer func() {
		_ = k.backend.BufferFinalize(paramsBuffer)
	}()

	outputBuffers, err := k.executable.Execute([]backends.Buffer{paramsBuffer}, nil, 0)
	if err != nil {
		return 0, nil, fmt.Errorf("execute gomlx kernel failed: %w", err)
	}
	defer func() {
		for _, output := range outputBuffers {
			_ = k.backend.BufferFinalize(output)
		}
	}()
	if len(outputBuffers) != 2 {
		return 0, nil, fmt.Errorf("unexpected output count: got=%d want=2", len(outputBuffers))
	}

	if err := k.backend.BufferToFlatData(outputBuffers[0], k.klFlat); err != nil {
		return 0, nil, fmt.Errorf("download kl output failed: %w", err)
	}
	if err := k.backend.BufferToFlatData(outputBuffers[1], k.gradFlat); err != nil {
		return 0, nil, fmt.Errorf("download gradient output failed: %w", err)
	}

	kl := k.klFlat[0]
	if math.IsNaN(kl) || math.IsInf(kl, 0) {
		return 0, nil, fmt.Errorf("invalid kl output: %v", kl)
	}

	return kl, k.gradFlat, nil
}

func (k *goMLXKLDivergenceKernel) Close() {
	if k.executable != nil {
		k.executable.Finalize()
	}
	if k.backend != nil {
		k.backend.Finalize()
	}
}

package indices

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/normalize"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"golang.org/x/sync/errgroup"
)

// 从叶子节点nodes由下至上通过合并的方式构建树结构
// 得到的树结构最少都有两层
//
// 由于ChunkPos的分配机制 得到的DocTree中Root的ChunkPos是所有节点中最小的
func (b *DocTreeBuilder) MergeBuild(ctx context.Context, nodes []*DocTreeNode) (*DocTree, error) {
	if len(nodes) == 0 {
		return nil, errors.ErrParams.Msg("build doc tree nodes are empty")
	}

	const maxLevel = 15 // 最多构建16层的树

	allNodes := make([]*DocTreeNode, 0, len(nodes)+maxLevel-1)
	allNodes = append(allNodes, nodes...)

	docTree := &DocTree{
		height: 1,
	}

	var (
		err        error
		childNodes = nodes // level-0
		forceRoot  = false
	)

	curPos := -1
	for level := 1; level <= maxLevel; level++ {
		childNodes, err = b.buildParentNodes(ctx, childNodes, level, forceRoot)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrInner, "build parent nodes failed, err=%v", err)
		}

		docTree.height++
		allNodes = append(allNodes, childNodes...)
		// 给所有分配新节点pos
		for _, node := range childNodes {
			node.pos = curPos
			node.core.ChunkPos = int32(curPos)
			curPos--
		}

		// 如果只返回了一个节点了，就认为已经构建得到根节点了
		if len(childNodes) == 1 {
			docTree.root = childNodes[0]
			break
		}

		if level == maxLevel-1 {
			// 倒数第二轮还没有收敛 强行收敛
			forceRoot = true
		}
	}

	docTree.nodes = allNodes

	return docTree, nil
}

// 从nodes构建上一层节点
//
// 构建规则：
//   - 对nodes中每个节点的embedding降维 随后进行聚类 聚类的结果生成一个父节点 父节点的content由LLM生成summary,
//   - 父节点的embeddding为summary的embedding
func (b *DocTreeBuilder) buildParentNodes(
	ctx context.Context,
	nodes []*DocTreeNode,
	targetLevel int,
	forceRoot bool,
) ([]*DocTreeNode, error) {
	// 粗略确定需要聚成几类
	targetNumOfClusters := -1
	numNodes := len(nodes)
	if numNodes <= 5 || forceRoot {
		// 节点太少就直接聚成一类的
		targetNumOfClusters = 1
	} else if numNodes <= 10 {
		targetNumOfClusters = 2
	} else if numNodes <= 50 {
		targetNumOfClusters = -1 // 尝试自动确定
	} else if numNodes <= 100 {
		targetNumOfClusters = numNodes / 10
	}

	var clusters [][]*DocTreeNode
	if targetNumOfClusters == 1 {
		clusters = append(clusters, nodes)
	} else {
		var err error
		clusters, err = b.performNodeClustering(targetNumOfClusters, nodes)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrInner, "perform node clustering failed, err=%v", err)
		}
	}

	var (
		mu       sync.Mutex
		newNodes []*DocTreeNode = make([]*DocTreeNode, 0, len(clusters))
	)

	eg, ctx := errgroup.WithContext(ctx)
	for _, clusterNodes := range clusters {
		eg.Go(func() error {
			newNode, err := b.extractNodes(ctx, clusterNodes, targetLevel)
			if err != nil {
				slog.ErrorContext(ctx, "extract nodes failed", slog.Any("err", err))
				return err
			}

			if newNode == nil {
				return nil
			}

			mu.Lock()
			newNodes = append(newNodes, newNode)
			mu.Unlock()
			return nil
		})
	}
	err := eg.Wait()
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "extract nodes failed, err=%v", err)
	}

	return newNodes, nil
}

// 降维+聚类
func (b *DocTreeBuilder) performNodeClustering(
	targetNumOfClusters int, // -1 means auto select, >=1 means fixed number
	nodes []*DocTreeNode,
) ([][]*DocTreeNode, error) {
	const (
		autoMinClusters       = 1
		autoMaxClusters       = 10
		targetDim             = 32
		defaultUMAPNNeighbors = 15
	)
	if len(nodes) <= 2 {
		return [][]*DocTreeNode{nodes}, nil
	}

	// [
	// 	[xxx, xxx, xxx],
	//  [xxx, xxx, xxx],
	//  [xxx, xxx, xxx],
	// ]
	nodesEmbeddings := make([][]float64, len(nodes))
	for idx, node := range nodes {
		nodesEmbeddings[idx] = slices.CastFloat[float32, float64](node.core.Embedding)
	}

	var normNodesEmbedding [][]float64
	normNodesEmbedding, err := normalize.L2(nodesEmbeddings)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "normalize failed, err=%v", err)
	}

	// 先降维
	nNeighbors := defaultUMAPNNeighbors
	if upperBound := len(nodes) - 1; nNeighbors > upperBound {
		nNeighbors = upperBound
	}
	umap, err := manifold.NewUMAP(
		targetDim,
		manifold.WithUMAPMetric("cosine"),
		manifold.WithUMAPNNeighbors(nNeighbors),
	)
	if err != nil {
		return nil, err
	}

	reductedEmbeedings, err := umap.FitTransform(normNodesEmbedding)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "fit transform failed, err=%v", err)
	}
	nSamples := len(reductedEmbeedings)

	var evaluation mixture.Evaluation

	if targetNumOfClusters == -1 {
		maxClusters := min(autoMaxClusters, nSamples)
		_, evaluation, _, err = mixture.AutoSelectGaussianMixture(
			reductedEmbeedings,
			autoMinClusters,
			maxClusters,
			mixture.AutoSelectionCriterionBIC,
		)
	} else {
		if targetNumOfClusters > nSamples {
			targetNumOfClusters = nSamples
		}
		var gmm *mixture.GaussianMixture
		gmm, err = mixture.NewGaussianMixture(targetNumOfClusters)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrInner, "new gaussian mixture failed, err=%v", err)
		}
		evaluation, err = gmm.FitEvaluate(reductedEmbeedings)
	}
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "fit evaluate failed, err=%v", err)
	}

	labels := evaluation.Labels // embeddings对应的聚类索引
	numOfClusters := slices.UniqueCount(labels)
	clusters := make([][]*DocTreeNode, numOfClusters)
	// 分开每一类的节点
	for embedIndex, clusterIndex := range labels {
		clusters[clusterIndex] = append(clusters[clusterIndex], nodes[embedIndex])
	}

	return clusters, nil
}

// 提取nodes中的内容 生成summary+embedding
func (b *DocTreeBuilder) extractNodes(
	ctx context.Context,
	nodes []*DocTreeNode,
	targetLevel int,
) (*DocTreeNode, error) {
	tbd := strings.Builder{}
	for _, node := range nodes {
		tbd.WriteString(node.core.Content)
		tbd.WriteString("\n")
	}
	summary, err := b.summarizer.Summarize(ctx, tbd.String())
	if err != nil {
		return nil, errors.WithMessagef(err, "generate summary failed")
	}

	// gen embedding
	embedResp, err := b.embedder.EmbedStrings(ctx, []string{summary})
	if err != nil {
		return nil, errors.Wrapf(errors.ErrEmbed, "embed summary failed, err=%v", err)
	}
	if len(embedResp) == 0 {
		return nil, errors.Wrapf(errors.ErrInner, "embed summary failed, embedResp is empty")
	}
	embedding := embedResp[0]

	var (
		notebookId  = nodes[0].core.NotebookId
		sourceId    = nodes[0].core.SourceId
		owner       = nodes[0].core.Owner
		derivation = collectDerivationFromChildren(nodes)
	)

	newNode := &DocTreeNode{
		core: &vschema.SourceDoc{
			Id:         uuid.NewV4().String(),
			NotebookId: notebookId,
			SourceId:   sourceId,
			Owner:      owner,
			Content:    summary,
			Embedding:  slices.CastFloat[float64, float32](embedding),
			ChunkPos:   -1, // 派生节点后续流程会分配
		},
		level:      targetLevel,
		pos:        -1,
		children:   nodes,
		derivation: derivation,
	}

	return newNode, nil
}

func collectDerivationFromChildren(nodes []*DocTreeNode) []string {
	d := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		d = append(d, node.derivation...)
	}
	return slices.Unique(d)
}

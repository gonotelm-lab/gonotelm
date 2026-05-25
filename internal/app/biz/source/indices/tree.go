package indices

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/manifold"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/mixture"
	"github.com/gonotelm-lab/gonotelm/pkg/algo/normalize"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"golang.org/x/sync/errgroup"

	einoembed "github.com/cloudwego/eino/components/embedding"
	einoschema "github.com/cloudwego/eino/schema"
)

type DocTreeNode struct {
	core  *vschema.SourceDoc
	level int // 0-based, leaf is 0

	// 表示文档在来源中所处的位置
	// pos < 0 表示为派生节点
	// pos >= 0 表示为原始来源文档的切块 (pos>=0表示为叶子节点)
	// 和core.ChunkPos 一致
	pos      int
	children []*DocTreeNode

	// 衍生自哪些叶子节点
	// 如果是叶子节点 这个字段就是自己的id
	derivedFrom []string
}

func NewDocTreeNode(
	core *vschema.SourceDoc,
	level int,
	pos int,
	children []*DocTreeNode,
	derivedFrom []string,
) *DocTreeNode {
	return &DocTreeNode{
		core:        core,
		level:       level,
		pos:         pos,
		children:    children,
		derivedFrom: derivedFrom,
	}
}

func (n *DocTreeNode) Core() *vschema.SourceDoc {
	if n == nil {
		return nil
	}
	return n.core
}

func (n *DocTreeNode) Level() int {
	if n == nil {
		return 0
	}
	return n.level
}

func (n *DocTreeNode) Pos() int {
	if n == nil {
		return -1
	}
	return n.pos
}

func (n *DocTreeNode) IsLeaf() bool {
	if n == nil {
		return false
	}
	return n.pos >= 0
}

func (n *DocTreeNode) Children() []*DocTreeNode {
	if n == nil {
		return nil
	}
	return n.children
}

func (n *DocTreeNode) DerivedFrom() []string {
	if n == nil {
		return nil
	}
	return n.derivedFrom
}

type (
	LLMProviderSelector func(ctx context.Context) string
	LLMModelSelector    func(ctx context.Context) string
)

type DocTreeBuilder struct {
	embedder einoembed.Embedder
	gateway  *gateway.Gateway

	providerSelector LLMProviderSelector
	modelSelector    LLMModelSelector
}

func NewDocTreeBuilder(
	embedder einoembed.Embedder,
	gateway *gateway.Gateway,
	providerSelector LLMProviderSelector,
	modelSelector LLMModelSelector,
) *DocTreeBuilder {
	return &DocTreeBuilder{
		embedder:         embedder,
		gateway:          gateway,
		providerSelector: providerSelector,
		modelSelector:    modelSelector,
	}
}

type DocTree struct {
	root   *DocTreeNode
	nodes  []*DocTreeNode // all nodes in the tree including root
	height int
}

func (t *DocTree) Nodes() []*DocTreeNode {
	if t == nil {
		return nil
	}
	return t.nodes
}

func (t *DocTree) SetNodes(nodes []*DocTreeNode) {
	if t == nil {
		return
	}

	t.nodes = nodes
}

func (t *DocTree) SetRoot(root *DocTreeNode) {
	if t == nil {
		return
	}

	t.root = root
}

func (t *DocTree) SetHeight(height int) {
	if t == nil {
		return
	}

	t.height = height
}

func (t *DocTree) Root() *DocTreeNode {
	if t == nil {
		return nil
	}
	return t.root
}

func (t *DocTree) Height() int {
	if t == nil {
		return 0
	}
	return t.height
}

// 从叶子节点nodes由下至上构建树结构
// 得到的树结构最少都有两层
// 
// 由于ChunkPos的分配机制 得到的DocTree中Root的ChunkPos是所有节点中最小的
func (b *DocTreeBuilder) Build(ctx context.Context, nodes []*DocTreeNode) (*DocTree, error) {
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
	providerType := chat.Type(b.providerSelector(ctx))
	provider, err := b.gateway.GetProvider(providerType)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "get provider failed, err=%v", err)
	}
	model := b.modelSelector(ctx)
	llmOption := chat.BuildLLMModelOption(model)
	tbd := strings.Builder{}
	for _, node := range nodes {
		tbd.WriteString(node.core.Content)
		tbd.WriteString("\n")
	}
	msg, err := prompts.SummarizePromptMessage(ctx, tbd.String(), "")
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "render summarize prompt failed, err=%v", err)
	}
	genResp, err := provider.Generate(ctx, []*einoschema.Message{msg}, llmOption)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate summary failed, err=%v", err)
	}

	summary := genResp.Content
	// gen embedding
	embedResp, err := b.embedder.EmbedStrings(ctx, []string{summary})
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "embed summary failed, err=%v", err)
	}
	if len(embedResp) == 0 {
		return nil, errors.Wrapf(errors.ErrInner, "embed summary failed, embedResp is empty")
	}
	embedding := embedResp[0]

	var (
		notebookId  = nodes[0].core.NotebookId
		sourceId    = nodes[0].core.SourceId
		owner       = nodes[0].core.Owner
		derivedFrom []string
	)

	for _, node := range nodes {
		derivedFrom = append(derivedFrom, node.derivedFrom...)
	}
	derivedFrom = slices.Unique(derivedFrom)

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
		level:       targetLevel,
		pos:         -1,
		children:    nodes,
		derivedFrom: derivedFrom,
	}

	return newNode, nil
}

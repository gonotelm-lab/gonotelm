package types

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxrepo "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	workerrepo "github.com/gonotelm-lab/gonotelm/internal/domain/worker/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	infrasandbox "github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// DataKeyRequest 是各产物 pipeline 在 pipeline.Data 中存放 *Request 的统一键。
const DataKeyRequest = "artifact.request"

// RequestFrom 从 pipeline.Data 取出当前产物的生成请求。
func RequestFrom(data *pipeline.Data) *Request {
	return pipeline.Get[*Request](data, DataKeyRequest)
}

// NewPipelineData 创建 pipeline.Data 并写入当前产物的生成请求。
func NewPipelineData(req *Request) *pipeline.Data {
	data := pipeline.NewData()
	data.Set(DataKeyRequest, req)
	return data
}

type Request struct {
	ArtifactId valobj.Id
	NotebookId valobj.Id
	UserId     valobj.Uid
	SourceIds  []valobj.Id
	Kind       artifactentity.Kind
	Payload    artifactentity.Payload
}

type Response struct {
	Title      string
	Result     []byte
	ResultKind artifactentity.ResultKind
}

type WorkerDeps struct {
	Agentize             *agentize.Service
	LLMGateway           *chat.Gateway
	Text2Image           *text2image.Text2ImageGateway
	Text2Audio           *text2audio.Text2AudioGateway
	Sandbox              *infrasandbox.Gateway
	SandboxRepository    sandboxrepo.Repository
	DistLock             adapter.DistributedLock
	ObjectStorage        adapter.ObjectStore
	KeyFactory           adapter.StoreKeyFactory
	CheckpointRepository workerrepo.CheckpointRepository
}

type Generator interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

type SessionState struct {
	NotebookId valobj.Id
	SourceIds  []valobj.Id
	UserId     valobj.Uid
	Lang       string
}

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
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
)

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
	ObjectStorage        storage.Storage
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

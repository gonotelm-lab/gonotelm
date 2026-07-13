package types

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	workerrepo "github.com/gonotelm-lab/gonotelm/internal/domain/worker/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
)

type Request struct {
	ArtifactId valobj.Id
	NotebookId valobj.Id
	UserId     string
	SourceIds  []valobj.Id
	Kind       artifactentity.Kind
	Payload    artifactentity.Payload
}

type Response struct {
	Title      string
	Result     []byte
	ResultKind artifactentity.ResultKind
}

type ServiceDeps struct {
	Agentize             *agentize.Service
	LLMGateway           *chat.Gateway
	Text2Image           *text2image.Text2ImageGateway
	ObjectStorage        storage.Storage
	CheckpointRepository workerrepo.CheckpointRepository
}

type Generator interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

type SessionState struct {
	NotebookId valobj.Id
	SourceIds  []valobj.Id
	UserId     string
	Lang       string
}

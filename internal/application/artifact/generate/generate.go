package generate

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
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
	Agentize      *agentize.Service
	LLMGateway    *chat.Gateway
	Text2Image    *text2image.Text2ImageGateway
	ObjectStorage storage.Storage
}

type Generator interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

func Run(ctx context.Context, deps *ServiceDeps, req *Request) (*Response, error) {
	g, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}
	return g.Generate(ctx, req)
}

func newGenerator(kind artifactentity.Kind, deps *ServiceDeps) (Generator, error) {
	switch kind {
	case artifactentity.KindMindmap:
		return &MindmapGenerator{deps: deps}, nil
	case artifactentity.KindReport:
		return &ReportGenerator{deps: deps}, nil
	case artifactentity.KindInfoGraphic:
		return &InfoGraphicGenerator{deps: deps}, nil
	case artifactentity.KindAudioOverview:
		return &AudioOverviewGenerator{deps: deps}, nil
	}
	return nil, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}

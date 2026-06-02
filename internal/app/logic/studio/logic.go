package studio

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cloudwego/eino/components/document"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/chunker/recursive"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"golang.org/x/sync/errgroup"
)

type Logic struct {
	objectStorage storage.Storage

	sourceBiz   *bizsource.Biz
	notebookBiz *biznotebook.Biz

	llmGateway *gateway.Gateway
	splitter   document.Transformer
}

func NewLogic(
	objectStorage storage.Storage,
	sourceBiz *bizsource.Biz,
	notebookBiz *biznotebook.Biz,
	llmGateway *gateway.Gateway,
) *Logic {
	splitter, err := recursive.NewSplitter(context.TODO(), &recursive.Config{
		ChunkSize: constants.MindmapMaxOnceToken,
		LenFunc:   token.Estimate,
	})
	if err != nil {
		panic(err)
	}

	return &Logic{
		objectStorage: objectStorage,
		sourceBiz:     sourceBiz,
		notebookBiz:   notebookBiz,
		llmGateway:    llmGateway,
		splitter:      splitter,
	}
}

func (l *Logic) helpGetNotebook(ctx context.Context, notebookId uuid.UUID) (*model.Notebook, error) {
	notebook, err := l.notebookBiz.GetNotebook(ctx, notebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", notebookId)
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	return notebook, nil
}

func (l *Logic) helpGetSourcesParsedContent(
	ctx context.Context,
	sources []*model.DecodedSource,
) (map[uuid.UUID]string, error) {
	var (
		mu       sync.Mutex
		contents map[uuid.UUID]string = make(map[uuid.UUID]string)
	)

	eg, wctx := errgroup.WithContext(ctx)
	for _, source := range sources {
		if source.ParsedContent == nil {
			slog.WarnContext(ctx, "source parsed content is nil", "source_id", source.Id)
			continue
		}

		if source.ParsedContent.StoreKey == "" {
			slog.WarnContext(ctx, "source parsed content store key is empty", "source_id", source.Id)
			continue
		}

		eg.Go(safe.Do(ctx, func() error {
			parsedContent, err := l.objectStorage.GetObject(wctx,
				&storage.GetObjectRequest{
					Key: source.ParsedContent.StoreKey,
				})
			if err != nil {
				return errors.WithMessage(err, "get parsed content failed")
			}

			mu.Lock()
			contents[source.Id] = pkgstring.FromBytes(parsedContent.Body)
			mu.Unlock()

			return nil
		}))
	}
	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return contents, nil
}

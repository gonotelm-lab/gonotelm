package studio

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"golang.org/x/sync/errgroup"
)

type CreateMindmapParams struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
}

func (l *Logic) CreateMindmap(
	ctx context.Context,
	params *CreateMindmapParams,
) error {
	// check notebook
	notebook, err := l.helpGetNotebook(ctx, params.NotebookId)
	if err != nil {
		return errors.WithMessage(err, "get notebook failed")
	}

	// check source ids ready
	sources, err := l.sourceBiz.BatchGetDecodedSources(
		ctx,
		notebook.Id,
		params.SourceIds,
	)
	if err != nil {
		return errors.WithMessage(err, "batch get decoded sources failed")
	}

	lenSources := len(sources)
	if lenSources == 0 {
		return errors.ErrParams.Msgf(
			"no sources found, notebook_id=%s, source_ids=%v",
			notebook.Id, params.SourceIds,
		)
	}

	var (
		parsedContents map[uuid.UUID][]byte
		mu             sync.Mutex
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

		eg.Go(func() error {
			parsedContent, err := l.objectStorage.GetObject(wctx,
				&storage.GetObjectRequest{
					Key: source.ParsedContent.StoreKey,
				})
			if err != nil {
				return errors.WithMessage(err, "get parsed content failed")
			}

			mu.Lock()
			parsedContents[source.Id] = parsedContent.Body
			mu.Unlock()

			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return err
	}

	if len(parsedContents) == lenSources {
		missingSourceIds := make([]uuid.UUID, 0, lenSources-len(parsedContents))
		for _, sourceId := range params.SourceIds {
			if _, ok := parsedContents[sourceId]; !ok {
				missingSourceIds = append(missingSourceIds, sourceId)
			}
		}

		return errors.ErrParams.Msgf(
			"some sources parsed content not found, notebook_id=%s, source_ids=%v, missing_source_ids=%v",
			notebook.Id, params.SourceIds,
			missingSourceIds,
		)
	}

	

	return nil
}

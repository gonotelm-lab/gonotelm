package source

import (
	"context"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const parsedContentStorePrefix = "parsed_file/"

func formatParsedContentStoreKey(sourceId, notebookId uuid.UUID) string {
	return parsedContentStorePrefix + notebookId.String() + "/" + sourceId.String()
}

func previewResponseContentType(mimeType string) string {
	switch mimeType {
	case model.MimeTypeText:
		return "text/plain; charset=utf-8"
	case model.MimeTypeMarkdown:
		return "text/markdown; charset=utf-8"
	default:
		return mimeType
	}
}

func buildNewSource(ctx context.Context, cmd *CreateSourceCommand) (*model.Source, error) {
	var (
		sourceId = uuid.NewV7()
		source   = &model.Source{
			Id:         sourceId,
			NotebookId: cmd.NotebookId,
			Kind:       cmd.Kind,
			Status:     model.SourceStatusInited, // all new sources are inited
			OwnerId:    cmd.OwnerId,
			UpdatedAt:  time.Now().UnixMilli(),
		}

		err error
	)

	switch cmd.Kind {
	case model.SourceKindText:
		ts := model.TextSourceContent{Text: cmd.TextContent}
		source.Content, err = sonic.Marshal(&ts)
		source.Title = truncateRunes(cmd.TextContent, 32)
	case model.SourceKindUrl:
		us := model.UrlSourceContent{Url: cmd.UrlContent.String()}
		source.Content, err = sonic.Marshal(&us)
		source.Title = us.Url
	case model.SourceKindFile:
		// file source inited with empty content
		source.Content = nil
		source.Title = ""
	default:
		return nil, errors.ErrParams.Msgf("invalid source kind: %s", cmd.Kind)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "marshal source failed, kind=%s, source_id=%s", cmd.Kind, sourceId)
	}

	return source, err
}

func truncateRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max])
}

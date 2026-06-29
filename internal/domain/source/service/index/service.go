package index

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
)

const (
	defaultChunkSize              = 800
	defaultOverlapSize            = 75
	defaultMaxSourceFileSizeBytes = 1024 * 1024 * 100 // 10MB
)

type Service struct {
	sourceConverters map[vo.SourceKind]convertdoc.Handler
	objectStorage    repository.FileObjectGetter
	sourceDocRepo    repository.SourceDocRepository
}

type ServiceConfig struct {
	DefaultChunkSize              int
	DefaultOverlapSize            int
	DefaultMaxSourceFileSizeBytes int64
}

func New(
	c ServiceConfig,
	objectStorage repository.FileObjectGetter,
	sourceDocRepo repository.SourceDocRepository,
) *Service {
	if c.DefaultChunkSize <= 0 {
		c.DefaultChunkSize = defaultChunkSize
	}
	if c.DefaultOverlapSize < 0 {
		c.DefaultOverlapSize = defaultOverlapSize
	}
	if c.DefaultMaxSourceFileSizeBytes <= 0 {
		c.DefaultMaxSourceFileSizeBytes = defaultMaxSourceFileSizeBytes
	}

	hc := convertdoc.HandlerConfig{
		ChunkSize:              c.DefaultChunkSize,
		OverlapSize:            c.DefaultOverlapSize,
		MaxSourceFileSizeBytes: c.DefaultMaxSourceFileSizeBytes,
	}

	s := &Service{
		sourceConverters: map[vo.SourceKind]convertdoc.Handler{
			vo.SourceKindText: convertdoc.NewTextHandler(hc),
			vo.SourceKindUrl:  convertdoc.NewUrlHandler(hc),
			vo.SourceKindFile: convertdoc.NewFileObjectHandler(hc, objectStorage),
		},
		objectStorage: objectStorage,
		sourceDocRepo: sourceDocRepo,
	}

	return s
}

type IndexSourceResult struct {
	SourceDocs        []*entity.SourceDoc
	ParsedContent     []byte
	ParsedContentType string
}

func (s *Service) IndexSource(
	ctx context.Context,
	source *entity.Source,
) (*IndexSourceResult, error) {
	slog.DebugContext(ctx, "now indexing source, first converting...", slog.String("source_id", source.Id.String()))
	result, err := s.convertSource(ctx, source)
	if err != nil {
		return nil, errors.WithMessage(err, "convert source failed")
	}

	estimatedToken := token.Estimate(pkgstring.FromBytes(result.ParsedContent))
	if estimatedToken > entity.MaxSourceTextContentToken {
		return nil, errors.Wrapf(domainerr.ErrSourceContentTooLong, "estimated token is %d", estimatedToken)
	}

	slog.DebugContext(ctx, "prepare source indices",
		slog.String("source_id", source.Id.String()),
		slog.Int("estimated_token", estimatedToken),
	)

	handleResult, err := s.convertSource(ctx, source)
	if err != nil {
		return nil, errors.WithMessage(err, "convert source failed")
	}

	sourceDocs := make([]*entity.SourceDoc, 0, len(handleResult.Docs))
	for idx, doc := range handleResult.Docs {
		sourceDoc, err := entity.NewSourceDoc(
			source.Id,
			source.NotebookId,
			source.OwnerId,
			idx,
			doc,
		)
		if err != nil {
			return nil, errors.WithMessage(err, "new source doc failed")
		}
		sourceDocs = append(sourceDocs, sourceDoc)
	}

	if err := s.sourceDocRepo.BatchSave(ctx, sourceDocs); err != nil {
		return nil, errors.WithMessage(err, "save source docs failed")
	}

	return &IndexSourceResult{
		SourceDocs:        sourceDocs,
		ParsedContent:     result.ParsedContent,
		ParsedContentType: result.ParsedContentType,
	}, nil
}

func (s *Service) convertSource(ctx context.Context, source *entity.Source) (*convertdoc.HandleResult, error) {
	converter, ok := s.sourceConverters[source.Kind]
	if !ok {
		return nil, errors.ErrInner.Msgf("can not convert for kind %s", source.Kind)
	}

	result, err := converter.Handle(ctx, source)
	if err != nil {
		return nil, errors.WithMessage(err, "handle source failed")
	}

	return result, nil
}

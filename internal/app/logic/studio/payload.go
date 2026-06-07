package studio

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type commonTaskParams struct {
	NotebookId uuid.UUID   `json:"notebook_id"`
	SourceIds  []uuid.UUID `json:"source_ids"`
}

type payloadUnmarshaler interface {
	extractSourceIds(payload []byte) ([]uuid.UUID, error)
}

var _ payloadUnmarshaler = &mindmapPayloadUnmarshaler{}

type mindmapPayloadUnmarshaler struct{}

func (m *mindmapPayloadUnmarshaler) extractSourceIds(payload []byte) ([]uuid.UUID, error) {
	var params generateMindmapTaskParams
	err := sonic.Unmarshal(payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal mindmap task params err=%v", err)
	}

	return params.SourceIds, nil
}

type reportPayloadUnmarshaler struct{}

func (r *reportPayloadUnmarshaler) extractSourceIds(payload []byte) ([]uuid.UUID, error) {
	var params generateReportTaskParams
	err := sonic.Unmarshal(payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal report task params err=%v", err)
	}

	return params.SourceIds, nil
}

var sourceIdsExtractors = map[model.ArtifactKind]payloadUnmarshaler{
	model.ArtifactKindMindmap: &mindmapPayloadUnmarshaler{},
	model.ArtifactKindReport:  &reportPayloadUnmarshaler{},
}

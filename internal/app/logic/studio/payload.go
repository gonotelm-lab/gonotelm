package studio

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

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

var sourceIdsExtractors = map[model.ArtifactKind]payloadUnmarshaler{
	model.ArtifactKindMindmap: &mindmapPayloadUnmarshaler{},
}

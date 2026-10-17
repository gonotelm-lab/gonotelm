// Package cleanup 负责删除后的补充清扫：notelm 注册一次性检查点，到期由 flow 回调本服务。
package cleanup

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	KindSource   = "source-cleanup"
	KindNotebook = "notebook-cleanup"
	KindArtifact = "artifact-cleanup"
)

// 回调路径写死；host 由配置提供。
const (
	SourceCallbackPath   = "/inner-api/v1/flow/webhook/source-cleanup"
	NotebookCallbackPath = "/inner-api/v1/flow/webhook/notebook-cleanup"
	ArtifactCallbackPath = "/inner-api/v1/flow/webhook/artifact-cleanup"
)

// Payload 是清理检查点的线协议载荷
type Payload struct {
	Kind       string    `json:"kind"`
	NotebookId valobj.Id `json:"notebook_id"`
	SourceId   valobj.Id `json:"source_id,omitzero"`
	ArtifactId valobj.Id `json:"artifact_id,omitzero"`
	// 记录删掉后要靠这里带出的 key 清对象
	Keys      []string      `json:"keys,omitempty"`
	Sources   []SourceRef   `json:"sources,omitempty"`
	Artifacts []ArtifactRef `json:"artifacts,omitempty"`
}

// SourceRef 是 notebook 清理需要的一个 source 及其对象 key。
type SourceRef struct {
	SourceId valobj.Id `json:"source_id"`
	Keys     []string  `json:"keys,omitempty"`
}

// ArtifactRef 是 notebook 清理需要的一个 artifact 及其对象 key。
type ArtifactRef struct {
	ArtifactId valobj.Id `json:"artifact_id"`
	Keys       []string  `json:"keys,omitempty"`
}

func (p *Payload) Marshal() ([]byte, error) {
	data, err := sonic.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "marshal cleanup payload failed")
	}

	return data, nil
}

func Unmarshal(data []byte) (*Payload, error) {
	var p Payload
	if err := sonic.Unmarshal(data, &p); err != nil {
		return nil, errors.Wrap(err, "unmarshal cleanup payload failed")
	}

	switch p.Kind {
	case KindSource:
		if p.SourceId.IsZero() {
			return nil, errors.ErrParams.Msg("source id is empty in cleanup payload")
		}
	case KindArtifact:
		if p.ArtifactId.IsZero() {
			return nil, errors.ErrParams.Msg("artifact id is empty in cleanup payload")
		}
	case KindNotebook:
	default:
		return nil, errors.ErrParams.Msgf("unknown cleanup kind: %s", p.Kind)
	}

	if p.NotebookId.IsZero() {
		return nil, errors.ErrParams.Msg("notebook id is empty in cleanup payload")
	}

	return &p, nil
}

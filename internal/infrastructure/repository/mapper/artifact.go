package mapper

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func ArtifactToSchema(a *artifactentity.Artifact) *schema.Artifact {
	var payloadBytes []byte
	if a.Payload != nil {
		b, err := sonic.Marshal(a.Payload)
		if err != nil {
			panic("marshal artifact payload: " + err.Error())
		}
		payloadBytes = b
	}
	return &schema.Artifact{
		Id:         uuid.UUID(a.Id),
		NotebookId: uuid.UUID(a.NotebookId),
		UserId:     a.UserId,
		Kind:       a.Kind.String(),
		Status:     a.Status.String(),
		FlowTaskId: a.FlowTaskId,
		Title:      a.Title,
		Result:     a.Result,
		ResultKind: a.ResultKind.String(),
		Payload:    payloadBytes,
		CreatedAt:  a.CreateTime.Value(),
		UpdatedAt:  a.UpdateTime.Value(),
	}
}

func ArtifactFromSchema(sch *schema.Artifact) (*artifactentity.Artifact, error) {
	a := &artifactentity.Artifact{
		Base: entity.Base{
			Id:         valobj.Id(sch.Id),
			CreateTime: valobj.NewTimeFromId(valobj.Id(sch.Id)),
			UpdateTime: valobj.NewTimeFrom(sch.UpdatedAt),
		},
		NotebookId: valobj.Id(sch.NotebookId),
		UserId:     sch.UserId,
		Kind:       artifactentity.Kind(sch.Kind),
		Status:     artifactentity.Status(sch.Status),
		FlowTaskId: sch.FlowTaskId,
		Title:      sch.Title,
		Result:     sch.Result,
		ResultKind: artifactentity.ResultKind(sch.ResultKind),
	}
	payload, err := decodePayload(a.Kind, sch.Payload)
	if err != nil {
		return nil, err
	}
	a.Payload = payload
	return a, nil
}

func decodePayload(kind artifactentity.Kind, b []byte) (artifactentity.Payload, error) {
	if len(b) == 0 {
		return nil, nil
	}
	switch kind {
	case artifactentity.KindMindmap:
		var p artifactentity.MindmapPayload
		if err := sonic.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindReport:
		var p artifactentity.ReportPayload
		if err := sonic.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindInfoGraphic:
		var p artifactentity.InfoGraphicPayload
		if err := sonic.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindAudioOverview:
		var p artifactentity.AudioOverviewPayload
		if err := sonic.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		return &p, nil
	}
	return nil, nil
}

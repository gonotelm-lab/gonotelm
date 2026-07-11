package mapper

import (
	"github.com/bytedance/sonic"
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
	}
}

func ArtifactFromSchema(sch *schema.Artifact) *artifactentity.Artifact {
	a := &artifactentity.Artifact{
		NotebookId: valobj.Id(sch.NotebookId),
		UserId:     sch.UserId,
		Kind:       artifactentity.Kind(sch.Kind),
		Status:     artifactentity.Status(sch.Status),
		FlowTaskId: sch.FlowTaskId,
		Title:      sch.Title,
		Result:     sch.Result,
		ResultKind: artifactentity.ResultKind(sch.ResultKind),
	}
	a.Base.Id = valobj.Id(sch.Id)
	a.Payload = decodePayload(a.Kind, sch.Payload)
	return a
}

func decodePayload(kind artifactentity.Kind, b []byte) artifactentity.Payload {
	switch kind {
	case artifactentity.KindMindmap:
		var p artifactentity.MindmapPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindReport:
		var p artifactentity.ReportPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindInfoGraphic:
		var p artifactentity.InfoGraphicPayload
		mustUnmarshal(b, &p)
		return &p
	case artifactentity.KindAudioOverview:
		var p artifactentity.AudioOverviewPayload
		mustUnmarshal(b, &p)
		return &p
	}
	return nil
}

func mustUnmarshal(b []byte, v any) {
	if len(b) == 0 {
		return
	}
	if err := sonic.Unmarshal(b, v); err != nil {
		panic("unmarshal artifact payload: " + err.Error())
	}
}

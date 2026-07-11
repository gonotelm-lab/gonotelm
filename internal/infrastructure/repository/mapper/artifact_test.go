package mapper

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactRoundTrip(t *testing.T) {
	notebookId := uuid.NewV7()
	sourceId := uuid.NewV7()
	payload := &artifactentity.MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{sourceId}}
	a, err := artifactentity.NewArtifact(notebookId, "u1", artifactentity.KindMindmap, payload)
	require.NoError(t, err)
	a.BindFlowTaskId("ft-1")

	sch := ArtifactToSchema(a)
	assert.Equal(t, "u1", sch.UserId)
	assert.Equal(t, "mindmap", sch.Kind)
	assert.Equal(t, "ft-1", sch.FlowTaskId)

	var rawPayload map[string]any
	require.NoError(t, sonic.Unmarshal(sch.Payload, &rawPayload))
	assert.Equal(t, notebookId.String(), rawPayload["notebook_id"])

	back := ArtifactFromSchema(sch)
	assert.Equal(t, a.Id, back.Id)
	assert.Equal(t, a.NotebookId, back.NotebookId)
	assert.Equal(t, a.Kind, back.Kind)
	assert.Equal(t, a.FlowTaskId, back.FlowTaskId)
	assert.Equal(t, a.Status, back.Status)
}

func TestArtifactRoundTrip_Kinds(t *testing.T) {
	notebookId := uuid.NewV7()
	cases := []struct {
		kind    artifactentity.Kind
		payload artifactentity.Payload
	}{
		{artifactentity.KindReport, &artifactentity.ReportPayload{NotebookId: notebookId}},
		{artifactentity.KindInfoGraphic, &artifactentity.InfoGraphicPayload{NotebookId: notebookId}},
		{artifactentity.KindAudioOverview, &artifactentity.AudioOverviewPayload{NotebookId: notebookId}},
	}
	for _, c := range cases {
		a, err := artifactentity.NewArtifact(notebookId, "u1", c.kind, c.payload)
		require.NoError(t, err)
		sch := ArtifactToSchema(a)
		back := ArtifactFromSchema(sch)
		assert.Equal(t, a.Kind, back.Kind, "kind=%s", c.kind)
		assert.Equal(t, a.NotebookId, back.NotebookId, "kind=%s", c.kind)
		assert.NotNil(t, back.Payload, "kind=%s", c.kind)
		assert.Equal(t, c.kind, back.Payload.Kind(), "kind=%s", c.kind)
	}
}

func TestArtifactRoundTrip_Time(t *testing.T) {
	notebookId := uuid.NewV7()
	a, err := artifactentity.NewArtifact(notebookId, "u1", artifactentity.KindMindmap, &artifactentity.MindmapPayload{NotebookId: notebookId})
	require.NoError(t, err)
	a.MarkCompleted([]byte("r"), artifactentity.ResultKindInline, "title")

	sch := ArtifactToSchema(a)
	assert.NotZero(t, sch.UpdatedAt)
	assert.NotZero(t, sch.CreatedAt)

	back := ArtifactFromSchema(sch)
	assert.Equal(t, a.UpdateTime.Value(), back.UpdateTime.Value())
	assert.Equal(t, a.CreateTime.Value(), back.CreateTime.Value())
}

func TestArtifactFromSchema_EmptyPayload(t *testing.T) {
	sch := &schema.Artifact{Id: uuid.NewV7(), Kind: "unknown"}
	back := ArtifactFromSchema(sch)
	assert.Nil(t, back.Payload)
}

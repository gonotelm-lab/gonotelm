package studio

import (
	"encoding/json"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

func TestPayload_GetNotebookID(t *testing.T) {
	p := &generateInfoGraphicTaskParams{
		commonTaskParams: &commonTaskParams{
			NotebookId: uuid.NewV7(),
		},
	}

	cc, _ := sonic.Marshal(p)

	var common commonTaskParams
	err := json.Unmarshal(cc, &common)
	if err != nil {
		t.Fatal(err)
	}

	if common.NotebookId != p.NotebookId {
		t.Fatal("notebook id mismatch")
	}

	t.Log(p, common)
}

func TestRecoverArtifactPayload(t *testing.T) {
	artifact := &Artifact{
		Id: uuid.NewV7(),
		NotebookId: uuid.NewV7(),
		Kind: model.ArtifactKindInfoGraphic,
		payload: []byte(`{"extra_prompt": "a beautiful image", "text_language": "en", "orientation": "landscape"}`),
	}
	 err := recoverArtifactPayload(artifact)
	if err != nil {
		t.Fatal(err)
	}

	if artifact.InfoGraphic == nil {
		t.Fatalf("artifact.InfoGraphic should not be nil")
	}
	if artifact.InfoGraphic.ExtraPrompt != "a beautiful image" {
		t.Fatalf("extra prompt not matched")
	}
}
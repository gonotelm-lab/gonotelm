package artifact

import (
	"encoding/json"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func buildWorkerInput(artifact *artifactentity.Artifact, payload json.RawMessage) generate.WorkerInput {
	return generate.WorkerInput{
		ArtifactId: artifact.Id.String(),
		NotebookId: artifact.NotebookId.String(),
		UserId:     artifact.UserId,
		SourceIds:  idsToStrings(artifact.Payload.GetSourceIds()),
		Kind:       string(artifact.Kind),
		Payload:    payload,
	}
}

func idsToStrings(ids []valobj.Id) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

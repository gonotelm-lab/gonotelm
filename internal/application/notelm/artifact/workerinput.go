package artifact

import (
	"encoding/json"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func buildWorkerInput(artifact *artifactentity.Artifact, payload json.RawMessage) contract.WorkerInput {
	return contract.WorkerInput{
		ArtifactId: artifact.Id.String(),
		NotebookId: artifact.NotebookId.String(),
		UserId:     artifact.UserId.String(),
		SourceIds:  valobj.IdsToStrings(artifact.Payload.GetSourceIds()),
		Kind:       string(artifact.Kind),
		Payload:    payload,
	}
}

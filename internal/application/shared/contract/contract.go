package contract

import "encoding/json"

// WorkerInput / WorkerOutput are the cross-process wire contract between
// notelm (submit + sync) and worker (generate) via flow task payload/result.
type WorkerInput struct {
	ArtifactId string          `json:"artifact_id"`
	NotebookId string          `json:"notebook_id"`
	UserId     string          `json:"user_id"`
	SourceIds  []string        `json:"source_ids"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"`
}

type WorkerOutput struct {
	Title      string `json:"title"`
	Result     []byte `json:"result"`
	ResultKind string `json:"result_kind"`
}

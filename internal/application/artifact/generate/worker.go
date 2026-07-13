package generate

import (
	"context"
	"encoding/json"
	"fmt"

	flowworker "github.com/gonotelm-lab/flow/client/worker"
	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/bytedance/sonic"
)

type WorkerInput struct {
	ArtifactId string          `json:"artifact_id"`
	NotebookId string          `json:"notebook_id"`
	UserId     string          `json:"user_id"`
	SourceIds  []string        `json:"source_ids"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"` // 不同的kind有不同的payload结构
}

type WorkerOutput struct {
	Title      string `json:"title"`
	Result     []byte `json:"result"`
	ResultKind string `json:"result_kind"`
}

func RegisterTypedWorker(client *flowworker.Client, deps *generatetypes.ServiceDeps) {
	flowworker.RegisterTyped(client, func(ctx context.Context, in WorkerInput) (WorkerOutput, error) {
		kind := artifactentity.Kind(in.Kind)
		if !kind.Supported() {
			return WorkerOutput{}, fmt.Errorf("unsupported artifact kind: %s", kind)
		}

		artifactId, err := parseId(in.ArtifactId)
		if err != nil {
			return WorkerOutput{}, fmt.Errorf("artifact_id: %w", err)
		}
		notebookId, err := parseId(in.NotebookId)
		if err != nil {
			return WorkerOutput{}, fmt.Errorf("notebook_id: %w", err)
		}
		sourceIds, err := parseIds(in.SourceIds)
		if err != nil {
			return WorkerOutput{}, fmt.Errorf("source_ids: %w", err)
		}
		payload, err := decodePayload(kind, in.Payload)
		if err != nil {
			return WorkerOutput{}, fmt.Errorf("payload: %w", err)
		}

		req := &generatetypes.Request{
			ArtifactId: artifactId,
			NotebookId: notebookId,
			UserId:     in.UserId,
			SourceIds:  sourceIds,
			Kind:       kind,
			Payload:    payload,
		}
		resp, err := Run(ctx, deps, req)
		if err != nil {
			return WorkerOutput{}, err
		}

		return WorkerOutput{
			Title:      resp.Title,
			Result:     resp.Result,
			ResultKind: string(resp.ResultKind),
		}, nil
	})
}

func decodePayload(kind artifactentity.Kind, raw json.RawMessage) (artifactentity.Payload, error) {
	switch kind {
	case artifactentity.KindMindmap:
		var p artifactentity.MindmapPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindReport:
		var p artifactentity.ReportPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindInfoGraphic:
		var p artifactentity.InfoGraphicPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindAudioOverview:
		var p artifactentity.AudioOverviewPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}

		return &p, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

func parseId(s string) (valobj.Id, error) {
	return valobj.NewIdFromString(s)
}

func parseIds(ss []string) ([]valobj.Id, error) {
	out := make([]valobj.Id, len(ss))
	for i, s := range ss {
		id, err := parseId(s)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out[i] = id
	}
	return out, nil
}

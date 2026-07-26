package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	flowworker "github.com/gonotelm-lab/flow/client/worker"
	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	errx "github.com/gonotelm-lab/multimodal/error"

	"github.com/bytedance/sonic"
)

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

func RegisterTypedWorker(client *flowworker.Client, deps *generatetypes.ServiceDeps) {
	flowworker.RegisterTypedResult(client, func(ctx context.Context, in WorkerInput) (flowworker.Result, error) {
		kind := artifactentity.Kind(in.Kind)
		if !kind.Supported() {
			return paramErrorResult("unsupported artifact kind: %s", kind), nil
		}

		artifactId, err := parseId(in.ArtifactId)
		if err != nil {
			return paramErrorResult("artifact_id: %v", err), nil
		}
		notebookId, err := parseId(in.NotebookId)
		if err != nil {
			return paramErrorResult("notebook_id: %v", err), nil
		}
		sourceIds, err := parseIds(in.SourceIds)
		if err != nil {
			return paramErrorResult("source_ids: %v", err), nil
		}
		payload, err := decodePayload(kind, in.Payload)
		if err != nil {
			return paramErrorResult("payload: %v", err), nil
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
			errMsg := err.Error()
			slog.Error("generate artifact failed",
				slog.String("error", errMsg),
				slog.Any("error_details", err),
				slog.String("artifact_id", artifactId.String()),
				slog.String("notebook_id", notebookId.String()),
				slog.String("payload", string(in.Payload)),
			)

			return flowworker.ErrorResult{
				Data:      []byte(errMsg),
				SkipRetry: shouldSkipRetry(err),
			}, nil
		}

		data, err := sonic.Marshal(WorkerOutput{
			Title:      resp.Title,
			Result:     resp.Result,
			ResultKind: string(resp.ResultKind),
		})
		if err != nil {
			return flowworker.ErrorResult{
				Data:      []byte(err.Error()),
				SkipRetry: true,
			}, nil
		}
		return flowworker.OkResult{Data: data}, nil
	})
}

func paramErrorResult(format string, args ...any) flowworker.Result {
	err := errors.ErrParams.Msgf(format, args...)
	return flowworker.ErrorResult{
		Data:      []byte(err.Error()),
		SkipRetry: true,
	}
}

func shouldSkipRetry(err error) bool {
	if errx.GetKind(err) != 0 {
		return !errx.IsRetryable(err)
	}
	if errors.Is(err, errors.ErrParams) || errors.Is(err, errors.ErrSerde) {
		return true
	}
	return false
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
	case artifactentity.KindFlashcard:
		var p artifactentity.FlashcardPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindQuiz:
		var p artifactentity.QuizPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case artifactentity.KindDataTable:
		var p artifactentity.DataTablePayload
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

package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"

	flow "github.com/gonotelm-lab/flow/client/worker"
	multimodalerr "github.com/gonotelm-lab/multimodal/error"

	"github.com/bytedance/sonic"
)

func RegisterTypedWorker(client *flow.Client, deps *generatetypes.ServiceDeps) {
	flow.RegisterTypedResult(client, func(ctx context.Context, in contract.WorkerInput) (flow.Result, error) {
		kind := artifactentity.Kind(in.Kind)
		if !kind.Supported() {
			return paramErrorResult("unsupported artifact kind: %s", kind), nil
		}

		artifactId, err := valobj.NewIdFromString(in.ArtifactId)
		if err != nil {
			return paramErrorResult("artifact_id: %v", err), nil
		}
		notebookId, err := valobj.NewIdFromString(in.NotebookId)
		if err != nil {
			return paramErrorResult("notebook_id: %v", err), nil
		}
		sourceIds, err := valobj.StringsToIds(in.SourceIds)
		if err != nil {
			return paramErrorResult("source_ids: %v", err), nil
		}
		userId, err := valobj.NewUidFromString(in.UserId)
		if err != nil {
			return paramErrorResult("user_id: %v", err), nil
		}
		payload, err := decodePayload(kind, in.Payload)
		if err != nil {
			return paramErrorResult("payload: %v", err), nil
		}

		req := &generatetypes.Request{
			ArtifactId: artifactId,
			NotebookId: notebookId,
			UserId:     userId,
			SourceIds:  sourceIds,
			Kind:       kind,
			Payload:    payload,
		}

		ctx = trace.RestoreReqIdFromTrace(ctx)
		ctx = pkgcontext.WithUserId(ctx, userId)
		resp, err := Run(ctx, deps, req)
		if err != nil {
			errMsg := err.Error()
			slog.ErrorContext(ctx, "worker generate artifact failed",
				slog.Any("err", err),
				slog.String("artifact_id", artifactId.String()),
				slog.String("notebook_id", notebookId.String()),
			)

			return flow.ErrorResult{
				Data:      []byte(errMsg),
				SkipRetry: shouldSkipRetry(err),
			}, nil
		}

		data, err := sonic.Marshal(contract.WorkerOutput{
			Title:      resp.Title,
			Result:     resp.Result,
			ResultKind: string(resp.ResultKind),
		})
		if err != nil {
			return flow.ErrorResult{
				Data:      []byte(err.Error()),
				SkipRetry: true,
			}, nil
		}
		return flow.OkResult{Data: data}, nil
	})
}

func paramErrorResult(format string, args ...any) flow.Result {
	err := errors.ErrParams.Msgf(format, args...)
	return flow.ErrorResult{
		Data:      []byte(err.Error()),
		SkipRetry: true,
	}
}

func shouldSkipRetry(err error) bool {
	if multimodalerr.GetKind(err) != 0 {
		return !multimodalerr.IsRetryable(err)
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
	case artifactentity.KindSlides:
		var p artifactentity.SlidesPayload
		if err := sonic.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

package studio

import (
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

type Artifact struct {
	Id         uuid.UUID
	NotebookId uuid.UUID
	Kind       model.ArtifactKind
	Status     model.ArtifactStatus
	ResultKind model.ArtifactResultKind
	UserId     string
	Result     []byte
	Title      string      // 大模型生成的产物的title
	SourceIds  []uuid.UUID // 生成时的来源id
	Timestamp  int64       // unix 时间戳

	// 按照resultKind的不同设置以下两个不同的字段
	Content string // inline

	ContentUrl  string // storage
	ContentKey  string // storage
	ContentType string

	// 一些类型的Artifact有特有的可自定义的参数 如下定义
	// 这些参数恢复是都是从payload中恢复

	// 信息图
	InfoGraphic *InfoGraphicExtrasParams

	payload []byte
}

func constructArtifact(task *model.ArtifactTask) (*Artifact, error) {
	a := &Artifact{
		Id:         task.Id,
		NotebookId: task.NotebookId,
		Kind:       task.Kind,
		Status:     task.Status,
		Title:      task.Title,
		Result:     task.Result,
		ResultKind: task.ResultKind,
		UserId:     task.UserId,
		Timestamp:  time.UnixMilli(task.UpdatedAt).Unix(), // use updated_at for timestamp

		payload: task.Payload,
	}

	var common commonTaskParams
	err := sonic.Unmarshal(task.Payload, &common)
	if err == nil {
		a.SourceIds = common.SourceIds
	} else {
		// log only
		slog.Warn("extract source_ids from payload failed",
			slog.Any("err", err),
			slog.String("task_id", task.Id.String()),
		)
	}

	if task.Status.Completed() {
		if task.ResultKind.Inline() {
			a.Content = string(task.Result)
		} else if task.ResultKind.Storage() {
			var storageResult model.ArtifactStorageResult
			if err := sonic.Unmarshal(task.Result, &storageResult); err == nil {
				a.ContentKey = storageResult.StoreKey
				a.ContentType = storageResult.ContentType
			}
		}
	}

	err = recoverArtifactPayload(a)
	if err != nil {
		slog.Error("recover artifact payload failed",
			slog.Any("err", err),
			slog.String("task_id", task.Id.String()),
		)
	}

	return a, nil
}

func recoverArtifactPayload(task *Artifact) error {
	switch task.Kind {
	case model.ArtifactKindInfoGraphic:
		err := sonic.Unmarshal(task.payload, &task.InfoGraphic)
		if err != nil {
			return errors.Wrapf(errors.ErrSerde, "unmarshal info graphic payload err=%v", err)
		}
	}

	return nil
}

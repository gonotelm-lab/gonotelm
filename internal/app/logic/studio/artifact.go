package studio

import (
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
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
	Content     string // inline
	ContentUrl  string // storage
	ContentKey  string // storage
	ContentType string
}

func constractArtifact(task *model.ArtifactTask) (*Artifact, error) {
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
	}

	sourceIds, err := sourceIdsExtractors[task.Kind].extractSourceIds(task.Payload)
	if err == nil {
		a.SourceIds = sourceIds
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
		} else {
			// TODO
		}
	}

	return a, nil
}

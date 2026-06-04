package studio

import (
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

	// 按照resultKind的不同设置以下两个不同的字段
	Content     string // inline
	ContentUrl  string // storage
	ContentKey  string // storage
	ContentType string
}

func NewArtifact(task *model.ArtifactTask) (*Artifact, error) {
	a := &Artifact{
		Id:         task.Id,
		NotebookId: task.NotebookId,
		Kind:       task.Kind,
		Status:     task.Status,
		Result:     task.Result,
		ResultKind: task.ResultKind,
		UserId:     task.UserId,
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

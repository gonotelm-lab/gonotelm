package model

import "github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"

type ArtifactKind string

const (
	ArtifactKindMindmap ArtifactKind = "mindmap"
	ArtifactKindReport  ArtifactKind = "report"
)

var validArtifactKinds = map[ArtifactKind]struct{}{
	ArtifactKindMindmap: {},
	ArtifactKindReport:  {},
}

func (k ArtifactKind) String() string {
	return string(k)
}

func (k ArtifactKind) Supported() bool {
	_, ok := validArtifactKinds[k]
	return ok
}

type ArtifactStatus string

const (
	ArtifactStatusPending   ArtifactStatus = "pending" // waiting for task to be claimed
	ArtifactStatusRunning   ArtifactStatus = "running"
	ArtifactStatusCompleted ArtifactStatus = "completed"
	ArtifactStatusFailed    ArtifactStatus = "failed"
	ArtifactStatusCancelled ArtifactStatus = "cancelled"
	ArtifactStatusExpired   ArtifactStatus = "expired"
)

func (s ArtifactStatus) String() string {
	return string(s)
}

func (s ArtifactStatus) Pending() bool {
	return s == ArtifactStatusPending
}

func (s ArtifactStatus) Running() bool {
	return s == ArtifactStatusRunning
}

func (s ArtifactStatus) Completed() bool {
	return s == ArtifactStatusCompleted
}

func (s ArtifactStatus) Failed() bool {
	return s == ArtifactStatusFailed
}

func (s ArtifactStatus) Cancelled() bool {
	return s == ArtifactStatusCancelled
}

func (s ArtifactStatus) Expired() bool {
	return s == ArtifactStatusExpired
}

type ArtifactResultKind string

const (
	ArtifactResultKindInline  ArtifactResultKind = "inline"
	ArtifactResultKindStorage ArtifactResultKind = "storage"
)

func (k ArtifactResultKind) String() string {
	return string(k)
}

func (k ArtifactResultKind) Inline() bool {
	return k == ArtifactResultKindInline
}

func (k ArtifactResultKind) Storage() bool {
	return k == ArtifactResultKindStorage
}

type ArtifactTask struct {
	Id         Id
	NotebookId Id
	Kind       ArtifactKind
	Status     ArtifactStatus
	Title      string
	Result     []byte
	ResultKind ArtifactResultKind
	UserId     string
	RunId      string
	LockNo     int32
	Payload    []byte
	CreatedAt  int64
	UpdatedAt  int64
	ExpiredAt  int64
}

func NewArtifactTaskFrom(task *schema.ArtifactTask) *ArtifactTask {
	return &ArtifactTask{
		Id:         task.Id,
		NotebookId: task.NotebookId,
		Kind:       ArtifactKind(task.Kind),
		Status:     ArtifactStatus(task.Status),
		Title:      task.Title,
		Result:     task.Result,
		ResultKind: ArtifactResultKind(task.ResultKind),
		UserId:     task.UserId,
		RunId:      task.RunId,
		LockNo:     task.LockNo,
		Payload:    task.Payload,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		ExpiredAt:  task.ExpiredAt,
	}
}

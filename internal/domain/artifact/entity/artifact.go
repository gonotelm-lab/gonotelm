package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Kind string

const (
	KindMindmap       Kind = "mindmap"
	KindReport        Kind = "report"
	KindInfoGraphic   Kind = "info_graphic"
	KindAudioOverview Kind = "audio_overview"
)

func (k Kind) Supported() bool {
	switch k {
	case KindMindmap, KindReport, KindInfoGraphic, KindAudioOverview:
		return true
	}
	return false
}

func (k Kind) String() string { return string(k) }

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

func (s Status) Pending() bool   { return s == StatusPending }
func (s Status) Running() bool   { return s == StatusRunning }
func (s Status) Completed() bool { return s == StatusCompleted }
func (s Status) Failed() bool    { return s == StatusFailed }
func (s Status) Cancelled() bool { return s == StatusCancelled }
func (s Status) String() string  { return string(s) }

func (s Status) IsTerminal() bool { return s.Completed() || s.Failed() || s.Cancelled() }

type ResultKind string

const (
	ResultKindInline  ResultKind = "inline"
	ResultKindStorage ResultKind = "storage"
)

func (r ResultKind) Inline() bool   { return r == ResultKindInline }
func (r ResultKind) Storage() bool  { return r == ResultKindStorage }
func (r ResultKind) String() string { return string(r) }

type Artifact struct {
	entity.Base
	NotebookId valobj.Id
	UserId     string
	Kind       Kind
	Status     Status
	FlowTaskId string
	Title      string
	Result     []byte
	ResultKind ResultKind
	Payload    Payload
}

func NewArtifact(notebookId valobj.Id, userId string, kind Kind, payload Payload) *Artifact {
	a := &Artifact{
		NotebookId: notebookId,
		UserId:     userId,
		Kind:       kind,
		Status:     StatusPending,
		Payload:    payload,
	}
	a.Base = entity.NewBase()
	return a
}

func (a *Artifact) IsOwner(userId string) bool { return a.UserId == userId }

func (a *Artifact) BindFlowTaskId(flowTaskId string) { a.FlowTaskId = flowTaskId }

func (a *Artifact) MarkRunning() { a.Status = StatusRunning }
func (a *Artifact) MarkCompleted(result []byte, kind ResultKind, title string) {
	a.Status = StatusCompleted
	a.Result = result
	a.ResultKind = kind
	a.Title = title
}
func (a *Artifact) MarkFailed() { a.Status = StatusFailed }

func (a *Artifact) MarkCancelled() { a.Status = StatusCancelled }

func (a *Artifact) MarkRetrying(newFlowTaskId string) {
	a.Status = StatusPending
	a.FlowTaskId = newFlowTaskId
	a.Title = ""
	a.Result = nil
	a.ResultKind = ""
}

func (a *Artifact) IsTerminal() bool {
	return a.Status.Completed() || a.Status.Failed() || a.Status.Cancelled()
}

func NewArtifactId() valobj.Id                     { return valobj.Id(uuid.NewV7()) }
func NewArtifactIdFromUUID(id valobj.Id) valobj.Id { return id }

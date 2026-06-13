package model

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
)

type ArtifactKind string

const (
	ArtifactKindMindmap     ArtifactKind = "mindmap"
	ArtifactKindReport      ArtifactKind = "report"
	ArtifactKindInfoGraphic ArtifactKind = "info_graphic"
)

var validArtifactKinds = map[ArtifactKind]struct{}{
	ArtifactKindMindmap:     {},
	ArtifactKindReport:      {},
	ArtifactKindInfoGraphic: {},
}

func (k ArtifactKind) String() string {
	return string(k)
}

func (k ArtifactKind) Supported() bool {
	_, ok := validArtifactKinds[k]
	return ok
}

type ArtifactInfoGraphicOrientation string

func (o ArtifactInfoGraphicOrientation) String() string {
	return string(o)
}

// portrait 竖版 / landscape 横版 / square 方形
const (
	ArtifactInfoGraphicOrientationPortrait  ArtifactInfoGraphicOrientation = "portrait"
	ArtifactInfoGraphicOrientationLandscape ArtifactInfoGraphicOrientation = "landscape"
	ArtifactInfoGraphicOrientationSquare    ArtifactInfoGraphicOrientation = "square"
)

func (o ArtifactInfoGraphicOrientation) Supported() bool {
	return o == ArtifactInfoGraphicOrientationPortrait ||
		o == ArtifactInfoGraphicOrientationLandscape ||
		o == ArtifactInfoGraphicOrientationSquare
}

func (o ArtifactInfoGraphicOrientation) ImageSize() string {
	switch o {
	case ArtifactInfoGraphicOrientationPortrait:
		return "768*1024"
	case ArtifactInfoGraphicOrientationLandscape:
		return "1024*768"
	case ArtifactInfoGraphicOrientationSquare:
		return "1024*1024"
	}

	return "1024*768"
}

type ArtifactInfoGraphicDetailLevel string

func (l ArtifactInfoGraphicDetailLevel) String() string {
	return string(l)
}

const (
	ArtifactInfoGraphicDetailLevelConcise  ArtifactInfoGraphicDetailLevel = "concise"
	ArtifactInfoGraphicDetailLevelStandard ArtifactInfoGraphicDetailLevel = "standard"
	ArtifactInfoGraphicDetailLevelDetailed ArtifactInfoGraphicDetailLevel = "detailed"
)

func (l ArtifactInfoGraphicDetailLevel) Supported() bool {
	return l == ArtifactInfoGraphicDetailLevelConcise ||
		l == ArtifactInfoGraphicDetailLevelStandard ||
		l == ArtifactInfoGraphicDetailLevelDetailed
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
	ResultKind ArtifactResultKind // inline/storage
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

type ArtifactInlineResult struct {
	Data []byte
}

type ArtifactStorageResult struct {
	StoreKey    string `json:"store_key"`
	ContentType string `json:"content_type"`
}

type FlavoredArtifactTask struct {
	*ArtifactTask

	InlineResult  *ArtifactInlineResult
	StorageResult *ArtifactStorageResult
}

func NewFlavoredArtifactTask(t *ArtifactTask) (*FlavoredArtifactTask, error) {
	at := &FlavoredArtifactTask{
		ArtifactTask: t,
	}

	// parse result
	switch t.ResultKind {
	case ArtifactResultKindInline:
		at.InlineResult = &ArtifactInlineResult{Data: t.Result}
	case ArtifactResultKindStorage:
		var r ArtifactStorageResult
		err := sonic.Unmarshal(at.Result, &r)
		if err != nil {
			return nil, err
		}

		at.StorageResult = &r
	}

	return at, nil
}

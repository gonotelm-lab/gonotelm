package model

type ArtifactKind string

const (
	ArtifactKindMindmap ArtifactKind = "mindmap"
)

var validArtifactKinds = map[ArtifactKind]struct{}{
	ArtifactKindMindmap: {},
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
	ArtifactStatusRunning   ArtifactStatus = "running"
	ArtifactStatusCompleted ArtifactStatus = "completed"
	ArtifactStatusFailed    ArtifactStatus = "failed"
	ArtifactStatusCancelled ArtifactStatus = "cancelled"
)

var validArtifactStatuses = map[ArtifactStatus]struct{}{
	ArtifactStatusRunning:   {},
	ArtifactStatusCompleted: {},
	ArtifactStatusFailed:    {},
	ArtifactStatusCancelled: {},
}

func (s ArtifactStatus) String() string {
	return string(s)
}

func (s ArtifactStatus) Supported() bool {
	_, ok := validArtifactStatuses[s]
	return ok
}

type ArtifactResultKind string

const (
	ArtifactResultKindInline  ArtifactResultKind = "inline"
	ArtifactResultKindStorage ArtifactResultKind = "storage"
)

type ArtifactTask struct {
	Id         Id
	NotebookId Id
	Kind       ArtifactKind
	Status     ArtifactStatus
	Result     []byte
	ResultKind string
	UserId     string
	RunId      string
	LockNo     int32
	Payload    []byte
	CreatedAt  int64
	UpdatedAt  int64
	ExpiredAt  int64
}

package vo

type SourceStatus string

func (s SourceStatus) String() string {
	return string(s)
}

const (
	SourceStatusInited    SourceStatus = "inited"
	SourceStatusUploading SourceStatus = "uploading"
	SourceStatusPreparing SourceStatus = "preparing"
	SourceStatusReady     SourceStatus = "ready"
	SourceStatusFailed    SourceStatus = "failed"
)

func (s SourceStatus) IsInited() bool {
	return s == SourceStatusInited
}

func (s SourceStatus) IsUploading() bool {
	return s == SourceStatusUploading
}

func (s SourceStatus) IsPreparing() bool {
	return s == SourceStatusPreparing
}

func (s SourceStatus) IsReady() bool {
	return s == SourceStatusReady
}

func (s SourceStatus) IsFailed() bool {
	return s == SourceStatusFailed
}

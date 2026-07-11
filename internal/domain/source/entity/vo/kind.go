package vo

type SourceKind string

const (
	SourceKindText SourceKind = "text"
	SourceKindUrl  SourceKind = "url"
	SourceKindFile SourceKind = "file"
)

func (s SourceKind) IsFile() bool {
	return s == SourceKindFile
}

func (s SourceKind) IsText() bool {
	return s == SourceKindText
}

func (s SourceKind) IsUrl() bool {
	return s == SourceKindUrl
}

func (s SourceKind) String() string {
	return string(s)
}

func (s SourceKind) Supported() bool {
	switch s {
	case SourceKindText, SourceKindUrl, SourceKindFile:
		return true
	}

	return false
}

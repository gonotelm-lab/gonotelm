package vo

type Style string

const (
	StyleDefault Style = "default"
	StyleAnalyst Style = "analyst"
	StyleGuide   Style = "guide"
)

func (s Style) String() string {
	return string(s)
}

func (s Style) IsValid() bool {
	switch s {
	case StyleDefault, StyleAnalyst, StyleGuide:
		return true
	default:
		return false
	}
}

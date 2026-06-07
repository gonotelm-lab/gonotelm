package context

type SceneType string

const (
	UnknownScene       = SceneType("unknown")
	StudioMindmapScene = SceneType("studio_mindmap")
	StudioReportScene  = SceneType("studio_report")
)

func (s SceneType) String() string {
	return string(s)
}

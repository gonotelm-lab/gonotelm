package context

type SceneType string

const (
	UnknownScene       = SceneType("unknown")
	ChatScene          = SceneType("chat")
	StudioMindmapScene = SceneType("studio.mindmap")
	StudioReportScene  = SceneType("studio.report")
)

func (s SceneType) String() string {
	return string(s)
}

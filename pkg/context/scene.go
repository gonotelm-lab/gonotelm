package context

type SceneType string

const (
	UnknownScene             = SceneType("unknown")
	ChatScene                = SceneType("chat")
	StudioMindmapScene       = SceneType("studio.mindmap")
	StudioReportScene        = SceneType("studio.report")
	StudioInfographicScene   = SceneType("studio.info_graphic")
	StudioAudioOverviewScene = SceneType("studio.audio_overview")
	StudioFlashcardScene     = SceneType("studio.flashcard")
	StudioQuizScene          = SceneType("studio.quiz")
	StudioDataTableScene     = SceneType("studio.data_table")
	StudioSlidesScene        = SceneType("studio.slides")
)

func (s SceneType) String() string {
	return string(s)
}

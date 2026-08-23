package context

type SceneType string

const (
	UnknownScene = SceneType("unknown")

	// chat scenario
	ChatScene           = SceneType("chat")
	ChatSuggestionScene = SceneType("chat.suggestion")

	// source scenario
	SourcePrepareScene = SceneType("source.prepare")

	// studio artifact scenario
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

// 每个场景的处理可能存在多次LLM调用
// SceneGroupId将多次同属一个场景下的属于同一次会话流程的LLM调用分组
// 分组定义如下：
// 
// 	1. studio生成artifact过程中 那么这些LLM调用属于同一组 artifactId为分组id
// 	2. 同一个会话下的所有调用属于同一个分组 chatId作为分组id
// 	3. 来源处理时同一个来源下的所有调用属于同一个分组 sourceId 作为分组id
type SceneGroupId = string

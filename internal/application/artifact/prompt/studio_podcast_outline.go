package prompt

type PodcastStyle string

const (
	PodcastStyleDeepResearch PodcastStyle = "deep-research"
	PodcastStyleAbstract     PodcastStyle = "abstract"
	PodcastStyleDiscussion   PodcastStyle = "discussion"
	PodcastStyleDebate       PodcastStyle = "debate"
)

type PodcastInfo struct {
	Style         PodcastStyle
	Description   string
	Speakers      []StudioPodcastSpeaker
	NumOfSegments int
}

var builtinPodcastSpeakers = map[string]StudioPodcastSpeaker{
	"John": {
		Name:        "John",
		Personality: "Analytical, detail-oriented, calm",
		Bio: "Former academic researcher with 15 years of experience in data synthesis and evidence-based reporting. " +
			"Known for breaking down complex topics into clear, actionable insights.",
	},
	"Linda": {
		Name:        "Linda",
		Personality: "Inquisitive, thorough, methodical",
		Bio: "Data scientist and investigative journalist. " +
			"Specializes in cross-referencing sources and uncovering hidden patterns in large datasets.",
	},

	"Amy": {
		Name:        "Amy",
		Personality: "Concise, clear, energetic",
		Bio: "Veteran news anchor and briefing specialist. " +
			"Expert at distilling lengthy reports into 2-minute summaries without losing core meaning.",
	},
	"Mark": {
		Name:        "Mark",
		Personality: "Direct, punchy, focused",
		Bio: "Former radio host with a knack for quick takes. " +
			"Known for delivering the essence of any story in under 60 seconds.",
	},

	"Sophia": {
		Name:        "Sophia",
		Personality: "Engaging, curious, balanced",
		Bio: "Experienced talk show host and moderator. " +
			"Skilled at facilitating multi-perspective conversations and drawing out diverse viewpoints.",
	},
	"David": {
		Name:        "David",
		Personality: "Thoughtful, diplomatic, probing",
		Bio: "Political analyst and panel moderator." +
			" Expertise in bridging opposing views and guiding constructive dialogue.",
	},

	"James": {
		Name:        "James",
		Personality: "Sharp, impartial, articulate",
		Bio: "Seasoned debate moderator and legal analyst. " +
			"Adept at structuring arguments, managing rebuttals, and ensuring fair conclusions.",
	},
	"Olivia": {
		Name:        "Olivia",
		Personality: "Assertive, quick-witted, principled",
		Bio: "Former competitive debater and ethics lecturer. " +
			"Passionate about rigorous logical reasoning and evidence-based counterarguments.",
	},
}

var builtinPodcastInfos = map[PodcastStyle]PodcastInfo{
	PodcastStyleDeepResearch: {
		Style:         PodcastStyleDeepResearch,
		Description:   "Deep research on the topic with source-based analysis, key evidence, and practical takeaways",
		Speakers:      []StudioPodcastSpeaker{builtinPodcastSpeakers["John"], builtinPodcastSpeakers["Linda"]},
		NumOfSegments: 7,
	},
	PodcastStyleAbstract: {
		Style:         PodcastStyleAbstract,
		Description:   "Short briefing on the topic covering the core idea, context, and main conclusion",
		Speakers:      []StudioPodcastSpeaker{builtinPodcastSpeakers["Amy"]},
		NumOfSegments: 3,
	},
	PodcastStyleDiscussion: {
		Style:         PodcastStyleDiscussion,
		Description:   "Multiple perspectives discussion on the topic with viewpoint comparison and trade-off analysis",
		Speakers:      []StudioPodcastSpeaker{builtinPodcastSpeakers["Sophia"], builtinPodcastSpeakers["David"], builtinPodcastSpeakers["James"]},
		NumOfSegments: 6,
	},
	PodcastStyleDebate: {
		Style:         PodcastStyleDebate,
		Description:   "Two opposing sides debate on the topic with arguments, rebuttals, and a balanced wrap-up",
		Speakers:      []StudioPodcastSpeaker{builtinPodcastSpeakers["James"], builtinPodcastSpeakers["Olivia"]},
		NumOfSegments: 5,
	},
}

type StudioPodcastSpeaker struct {
	Name        string
	Personality string
	Bio         string
}

type StudioPodcastOutlineTemplateVars struct {
	SourceIds     []string
	Speakers      []StudioPodcastSpeaker
	Tips          string
	NumOfSegments int
	Language      string
	Style         PodcastStyle
	StyleDesc     string
}

func (v StudioPodcastOutlineTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds":     normalizeStrings(v.SourceIds),
		"Speakers":      v.Speakers,
		"Tips":          v.Tips,
		"NumOfSegments": v.NumOfSegments,
		"Language":      v.Language,
		"StyleInfo": map[string]any{
			"Style":       v.Style,
			"Description": v.StyleDesc,
		},
	}
}

type StudioPodcastOutlineTemplate = template[StudioPodcastOutlineTemplateVars]

package entity

type Language string

const (
	LanguageAuto    Language = ""
	LanguageChinese Language = "zh-CN"
	LanguageEnglish Language = "en-US"
)

func (l Language) DisplayName() string {
	switch l {
	case LanguageChinese:
		return "中文"
	case LanguageEnglish:
		return "English"
	default:
		return "English"
	}
}

func (l Language) IsValid() bool {
	return l == LanguageChinese || l == LanguageEnglish
}

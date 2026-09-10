package text2audio

import (
	audios "github.com/gonotelm-lab/multimodal/audio"
	"github.com/gonotelm-lab/multimodal/audio/mimo"
	"github.com/gonotelm-lab/multimodal/audio/minimax"
	"github.com/gonotelm-lab/multimodal/audio/schema"
)

// WAVOption 返回强制提供商输出 WAV 的选项（qwen 默认即 WAV）。
func WAVOption(provider Text2AudioProvider) audios.Option {
	switch provider {
	case Text2AudioQwen:
		return nil
	case Text2AudioMimo:
		return mimo.WithFormat(mimo.FormatWAV)
	case Text2AudioMiniMax:
		return minimax.WithAudioFormat(minimax.AudioFormatWAV)
	}
	return nil
}

// AudioLang 将语言代码映射为 TTS schema 语言。
func AudioLang(lang string) schema.Language {
	switch lang {
	case "zh-CN":
		return schema.LanguageChinese
	case "en-US":
		return schema.LanguageEnglish
	default:
		return schema.LanguageAuto
	}
}

package text2audio

import (
	"unicode"

	"github.com/gonotelm-lab/multimodal/audio/schema"
)

// ResolveLineLanguage 按口播/对白这一行的实际内容决定 TTS 朗读语种，
// 解决「中英混排时英文被按中文发音朗读」的问题：
//
//   - 纯中文行（含汉字、无拉丁字母）→ LanguageChinese，中文发音稳定；
//   - 中英混排行（汉字与拉丁字母并存）→ LanguageAuto：不锁定语种，
//     交给模型按句自动识别并中英混读，句中英文按英文发音；
//   - 纯英文行（无汉字）→ LanguageEnglish；
//   - 无字母行（纯数字 / 符号）→ 回退为 fallback（通常是整片的语言）。
func ResolveLineLanguage(text string, fallback schema.Language) schema.Language {
	hasHan, hasLatin := scanLineScripts(text)
	switch {
	case hasHan && !hasLatin:
		return schema.LanguageChinese
	case hasHan && hasLatin:
		return schema.LanguageAuto
	case !hasHan && hasLatin:
		return schema.LanguageEnglish
	default:
		return fallback
	}
}

// scanLineScripts 扫描文本是否含汉字、拉丁字母；两者都出现即提前返回。
func scanLineScripts(text string) (hasHan, hasLatin bool) {
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			hasHan = true
		} else if isLatinLetter(r) {
			hasLatin = true
		}
		if hasHan && hasLatin {
			return
		}
	}
	return
}

// isLatinLetter 基础拉丁字母（含 Latin-1 补充与拉丁扩展区），覆盖常见西欧拼写。
func isLatinLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x00C0 && r <= 0x024F)
}

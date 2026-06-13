package dashscope

import (
	"github.com/gonotelm-lab/gonotelm/pkg/text2image"
)

const (
	extraKeyNegativePrompt = "dashscope_negative_prompt"
	extraKeyPromptExtend   = "dashscope_prompt_extend"
	extraKeyWatermark      = "dashscope_watermark"
	extraKeyN              = "dashscope_n"
	extraKeySeed           = "dashscope_seed"

	paramNegativePrompt = "negative_prompt"
	paramPromptExtend   = "prompt_extend"
	paramWatermark      = "watermark"
	paramSize           = "size"
	paramN              = "n"
	paramSeed           = "seed"
)


// WithNegativePrompt 设置反向提示词，用于描述不希望在图像中出现的内容。
func WithNegativePrompt(negativePrompt string) text2image.Option {
	return text2image.WithExtra(extraKeyNegativePrompt, negativePrompt)
}

// WithPromptExtend 设置是否开启 Prompt 智能改写功能。
func WithPromptExtend(enable bool) text2image.Option {
	return text2image.WithExtra(extraKeyPromptExtend, enable)
}

// WithWatermark 设置是否在图像右下角添加水印。
func WithWatermark(enable bool) text2image.Option {
	return text2image.WithExtra(extraKeyWatermark, enable)
}

// WithN 设置输出图像的数量（仅 qwen-image-2.0 系列支持 1-6 张）。
func WithN(n int) text2image.Option {
	if n <= 0 {
		return nil
	}
	return text2image.WithExtra(extraKeyN, n)
}

// WithSeed 设置随机数种子，使生成内容保持相对稳定。
func WithSeed(seed int) text2image.Option {
	return text2image.WithExtra(extraKeySeed, seed)
}

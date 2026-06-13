package schema

type ResponseFormat string

const (
	ResponseFormatURL    ResponseFormat = "url"
	ResponseFormatBase64 ResponseFormat = "base64"
)

type Response struct {
	// 生成的图像的呈现形式
	ResponseFormat ResponseFormat

	// 生成的图像url 注意有效期
	ImageURL string

	// 生成的图像base64
	ImageBase64 string

	Extras map[string]any
}

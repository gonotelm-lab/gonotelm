package adapter

import "context"

// 理解图片内容 并且返回图片的文本描述
// 如果包含文字/图表等 返回的文字描述中会包含文字/图表内容
// 如果不包含文字等 则返回的文字只包含对图片的描述
type ImageInterpreter interface {
	// http url or data:image/jpeg;base64,<BASE64_DATA>
	Interpret(ctx context.Context, input string) (string, error)
	InterpretBytes(ctx context.Context, bytes []byte) (string, error)
}

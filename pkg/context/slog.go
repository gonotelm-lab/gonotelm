package context

import (
	"context"
	"log/slog"
)

const (
	AttrKeyUserID       = "user_id"
	AttrKeySceneType    = "scene"
	AttrKeyOperatorType = "operator"
)

func getUserIdSlogAttr(ctx context.Context) (slog.Attr, bool) {
	userId := GetUserId(ctx)
	if userId == "" {
		return slog.Attr{}, false
	}

	attr := slog.String(AttrKeyUserID, userId)
	return attr, true
}

func getSceneSlogAttr(ctx context.Context) (slog.Attr, bool) {
	sceneType := GetSceneType(ctx)
	if sceneType == UnknownScene {
		return slog.Attr{}, false
	}

	attr := slog.String(AttrKeySceneType, string(sceneType))
	return attr, true
}

func getOperatorSlogAttr(ctx context.Context) (slog.Attr, bool) {
	operatorType := GetOperatorType(ctx)
	if operatorType == OperatorTypeUser {
		return slog.Attr{}, false
	}

	attr := slog.String(AttrKeyOperatorType, string(operatorType))
	return attr, true
}

var defaultSlogAttrsExtractors = []SlogAttrExtractor{
	getUserIdSlogAttr,
	getSceneSlogAttr,
	getOperatorSlogAttr,
}

func ToSlogAttrs(ctx context.Context) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(defaultSlogAttrsExtractors)+len(slogAttrsExtractors))
	for _, defaultExtractor := range defaultSlogAttrsExtractors {
		if attr, ok := defaultExtractor(ctx); ok {
			attrs = append(attrs, attr)
		}
	}

	for _, extractor := range slogAttrsExtractors {
		if attr, ok := extractor(ctx); ok {
			attrs = append(attrs, attr)
		}
	}

	return attrs
}

type SlogAttrExtractor func(ctx context.Context) (slog.Attr, bool)

var slogAttrsExtractors = make([]SlogAttrExtractor, 0)

// 注册自定义的从context中提取slog.Attr的函数
// 建议程序初始化时只注册一次即可
func RegisterSlogAttrs(handlers ...SlogAttrExtractor) {
	slogAttrsExtractors = append(slogAttrsExtractors, handlers...)
}

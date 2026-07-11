package chat

import "context"

type (
	modelNameKeyType      struct{}
	semReleaseFuncKeyType struct{}
	isStreamingKeyType    struct{}
)

func withIsStreaming(ctx context.Context, isStreaming bool) context.Context {
	return context.WithValue(ctx, isStreamingKeyType{}, isStreaming)
}

func getIsStreaming(ctx context.Context) bool {
	isStreaming, ok := ctx.Value(isStreamingKeyType{}).(bool)
	if !ok {
		return false
	}

	return isStreaming
}

func withModelName(ctx context.Context, modelName string) context.Context {
	return context.WithValue(ctx, modelNameKeyType{}, modelName)
}

func getModelName(ctx context.Context) string {
	modelName, ok := ctx.Value(modelNameKeyType{}).(string)
	if !ok {
		return ""
	}

	return modelName
}

func withSemReleaseFunc(ctx context.Context, release func()) context.Context {
	if release == nil {
		return ctx
	}

	return context.WithValue(ctx, semReleaseFuncKeyType{}, release)
}

func getSemReleaseFunc(ctx context.Context) func() {
	release, ok := ctx.Value(semReleaseFuncKeyType{}).(func())
	if !ok || release == nil {
		return nil
	}

	return release
}

func runSemRelease(ctx context.Context) {
	release := getSemReleaseFunc(ctx)
	if release == nil {
		return
	}

	release()
}

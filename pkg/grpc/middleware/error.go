package middleware

import (
	"context"
	stderr "errors"

	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryError 将业务 *InnerError 映射为 InvalidArgument，message 为 InnerError JSON；
// 内部错误（含 Status>=500 的 InnerError、非 InnerError）映射为 Internal。
// 已是 gRPC status 的错误原样返回。
func UnaryError() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		return resp, mapError(err)
	}
}

func mapError(err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}

	cause := xerror.Cause(err)

	var ie *xerror.InnerError
	if stderr.As(cause, &ie) {
		msg := ie.Json()
		if ie.ShouldLogError() {
			return status.Error(codes.Internal, msg)
		}
		return status.Error(codes.InvalidArgument, msg)
	}

	return status.Error(codes.Internal, cause.Error())
}

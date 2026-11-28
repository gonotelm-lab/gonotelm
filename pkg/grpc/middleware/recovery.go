package middleware

import (
	"context"
	"log/slog"
	"runtime/debug"

	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryRecovery 捕获 panic，打日志后返回 Internal。
func UnaryRecovery() grpc.UnaryServerInterceptor {
	return recovery.UnaryServerInterceptor(
		recovery.WithRecoveryHandlerContext(
			func(ctx context.Context, p any) error {
				slog.ErrorContext(
					ctx,
					"grpc server panic",
					slog.Any("err", p),
					slog.String("stack", string(debug.Stack())),
				)
				return status.Error(codes.Internal, xerror.ErrInner.Json())
			},
		),
	)
}

package middleware

import "google.golang.org/grpc"

// UnaryServerInterceptors 默认服务端链：
// recovery → tracing → error → logging → validate。
// error 在 logging 外侧：logging 先看到原始 *InnerError，再由 error 映射为 gRPC status。
// validate 最内侧：校验失败也能被 logging 打到。
func UnaryServerInterceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		UnaryRecovery(),
		UnaryTracing(),
		UnaryError(),
		UnaryLogging(),
		UnaryValidate(),
	}
}

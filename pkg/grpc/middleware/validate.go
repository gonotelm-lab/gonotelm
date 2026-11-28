package middleware

import (
	"buf.build/go/protovalidate"
	protovalidatemid "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"google.golang.org/grpc"
)

var validator protovalidate.Validator

func init() {
	var err error
	validator, err = protovalidate.New(protovalidate.WithFailFast())
	if err != nil {
		panic(err)
	}
}

// UnaryValidate 校验入站 proto（buf.validate）；失败返回 InvalidArgument。
func UnaryValidate() grpc.UnaryServerInterceptor {
	return protovalidatemid.UnaryServerInterceptor(validator)
}

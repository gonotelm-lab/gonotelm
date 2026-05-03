package http

import (
	"errors"

	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func NewResult(code int, msg string, data any) *Result {
	return &Result{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

func NewOkResult(data any) *Result {
	return NewResult(0, "ok", data)
}

func resultFrom(e *xerror.InnerError) *Result {
	return &Result{
		Code: e.Code,
		Msg:  e.Message,
	}
}

func ResultFromError(err error) *Result {
	var e *xerror.InnerError
	if errors.As(err, &e) {
		return &Result{
			Code: e.Code,
			Msg:  e.Message,
		}
	}

	return &Result{
		Code: xerror.CodeDatabaseErr,
		Msg:  err.Error(),
	}
}

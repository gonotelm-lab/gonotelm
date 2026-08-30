package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	xstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type InnerError struct {
	Status  int    `json:"status,omitempty"` // http status code
	Code    int    `json:"code"`             // biz code
	Message string `json:"msg"`
}

func (e *InnerError) Error() string {
	if e == nil {
		return ""
	}

	return fmt.Sprintf("code=%d status=%d msg=%s", e.Code, e.Status, e.Message)
}

func (e *InnerError) Json() string {
	if e == nil {
		return ""
	}
	s, _ := json.Marshal(e)
	return xstring.FromBytes(s)
}

func (e *InnerError) Msg(msg string) *InnerError {
	if e == nil {
		return nil
	}

	return &InnerError{
		Code:    e.Code,
		Status:  e.Status,
		Message: msg,
	}
}

func (e *InnerError) Msgf(format string, args ...any) *InnerError {
	if e == nil {
		return nil
	}

	return &InnerError{
		Code:    e.Code,
		Status:  e.Status,
		Message: fmt.Sprintf(format, args...),
	}
}

func (e *InnerError) ExtMsg(extmsg string) *InnerError {
	// 保留原来msg的基础下 在msg中新增extmsg
	if e == nil {
		return nil
	}

	msg := e.Message + ": " + extmsg

	return &InnerError{
		Code:    e.Code,
		Status:  e.Status,
		Message: msg,
	}
}

func (e *InnerError) ErrCode(ecode int) *InnerError {
	if e == nil {
		return nil
	}

	return &InnerError{
		Code:    ecode,
		Status:  e.Status,
		Message: e.Message,
	}
}

func (e *InnerError) Is(err error) bool {
	if oth, ok := err.(*InnerError); ok {
		return e.Equal(oth)
	}
	return false
}

func (e *InnerError) Equal(other *InnerError) bool {
	if other == nil {
		return e == nil
	}
	if e == nil {
		return other == nil
	}

	// 不要求Msg相等
	return e.Code == other.Code && e.Status == other.Status
}

func (e *InnerError) ShouldLogError() bool {
	return e.Status >= http.StatusInternalServerError
}

func NewInnerError(httpStatus, code int, msg string) *InnerError {
	return &InnerError{
		Status:  httpStatus,
		Code:    code,
		Message: msg,
	}
}

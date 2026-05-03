package http

import (
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	RequestContextInnerErrKey = "gonotelm/hertz/inner/err"
	RequestContextRawErrKey   = "gonotelm/hertz/raw/err"
)

func OkResp(c *app.RequestContext, data any) {
	c.JSON(http.StatusOK, NewOkResult(data))
}

func ErrResp(c *app.RequestContext, err error) {
	cause := xerror.Cause(err)
	var ie *xerror.InnerError
	if errors.As(cause, &ie) {
		c.Set(RequestContextInnerErrKey, ie) // already is caused error
		c.Set(RequestContextRawErrKey, err)
		c.AbortWithStatusJSON(ie.Status, resultFrom(ie))
		return
	}

	c.Set(RequestContextRawErrKey, err) // raw error
	// can not convert to xerror.Error, return internal server error
	c.AbortWithStatusJSON(
		http.StatusInternalServerError,
		NewResult(xerror.CodeUnknownErr, cause.Error(), nil),
	)
}

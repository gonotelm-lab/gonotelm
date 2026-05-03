package http

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/go-playground/validator/v10"
)

type SelfValidator interface {
	Validate() error
}

type CanonicalBinder struct {
	impl binding.Binder
}

func uuidUnmarshaler(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
	parsed, err := uuid.ParseString(text)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(parsed), nil
}

func urlUnmarshaler(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
	parsed, err := url.ParseRequestURI(text)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(*parsed), nil
}

func registerTypeUnmarshalers(cfg *binding.BindConfig) {
	cfg.MustRegTypeUnmarshal(reflect.TypeFor[uuid.UUID](), uuidUnmarshaler)
	cfg.MustRegTypeUnmarshal(reflect.TypeFor[url.URL](), urlUnmarshaler)
}

func initValidator() *validator.Validate {
	vd := validator.New()
	return vd
}

// CanonicalBinder wraps any binding error to xerror.Error for standardization
func NewCanonicalBinder() *CanonicalBinder {
	vd := initValidator()

	cfg := binding.NewBindConfig()
	registerTypeUnmarshalers(cfg)

	cfg.ValidatorFunc = func(req *protocol.Request, v any) error {
		err := vd.Struct(v)
		if err != nil {
			return err
		}

		if selfValidator, ok := v.(SelfValidator); ok && selfValidator != nil {
			return selfValidator.Validate()
		}

		return nil
	}
	return &CanonicalBinder{
		impl: binding.NewDefaultBinder(cfg),
	}
}

var _ binding.Binder = (*CanonicalBinder)(nil)

func (b *CanonicalBinder) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// invalid params returns http.StatusOK
	return xerror.NewInnerError(http.StatusOK, xerror.CodeInvalidParams, err.Error())
}

func (b *CanonicalBinder) Name() string {
	return fmt.Sprintf("gonotelm:%s", b.impl.Name())
}

func (b *CanonicalBinder) Bind(req *protocol.Request, obj any, params param.Params) error {
	return b.wrapError(b.impl.Bind(req, obj, params))
}

func (b *CanonicalBinder) BindQuery(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.BindQuery(req, obj))
}

func (b *CanonicalBinder) BindHeader(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.BindHeader(req, obj))
}

func (b *CanonicalBinder) BindPath(req *protocol.Request, obj any, params param.Params) error {
	return b.wrapError(b.impl.BindPath(req, obj, params))
}

func (b *CanonicalBinder) BindForm(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.BindForm(req, obj))
}

func (b *CanonicalBinder) BindJSON(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.BindJSON(req, obj))
}

func (b *CanonicalBinder) BindProtobuf(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.BindProtobuf(req, obj))
}

func (b *CanonicalBinder) Validate(req *protocol.Request, obj any) error {
	return b.wrapError(b.impl.Validate(req, obj))
}

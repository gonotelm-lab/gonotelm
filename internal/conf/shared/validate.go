package shared

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	// required 打在非指针结构体上默认会被忽略，这里按 v11 的行为启用
	v := validator.New(validator.WithRequiredStructEnabled())
	// 报错用 toml 的 key，而不是 Go 字段名
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
		if name == "" || name == "-" {
			return field.Name
		}

		return name
	})

	return v
}

func Validate(cfg any) error {
	err := validate.Struct(cfg)
	if err == nil {
		return nil
	}

	var fieldErrs validator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return err
	}

	msgs := make([]string, 0, len(fieldErrs))
	for _, fieldErr := range fieldErrs {
		msgs = append(msgs, formatFieldError(fieldErr))
	}

	return errors.New("invalid config: " + strings.Join(msgs, ", "))
}

// formatFieldError 只报 key 与规则，不打印配置值（可能含密钥）
func formatFieldError(fieldErr validator.FieldError) string {
	rule := fieldErr.Tag()
	if param := fieldErr.Param(); param != "" {
		rule += "=" + param
	}

	return fmt.Sprintf("%s: %s", keyPath(fieldErr.Namespace()), rule)
}

// keyPath 去掉命名空间里的 Go 结构体名，只留 toml 层级：NotelmConfig.InfraConfig.redis.addrs -> redis.addrs
func keyPath(namespace string) string {
	parts := strings.Split(namespace, ".")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && !unicode.IsUpper(rune(part[0])) {
			kept = append(kept, part)
		}
	}

	if len(kept) == 0 {
		return namespace
	}

	return strings.Join(kept, ".")
}

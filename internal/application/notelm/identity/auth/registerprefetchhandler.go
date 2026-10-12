package auth

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// RegisterPrefetchHandler 注册前元数据：把服务端配置翻译成表单要用的字段。
type RegisterPrefetchHandler struct{}

func NewRegisterPrefetchHandler() *RegisterPrefetchHandler { return &RegisterPrefetchHandler{} }

type RegisterPrefetchResult struct {
	RequireComplexPassword bool
}

func (h *RegisterPrefetchHandler) Handle() *RegisterPrefetchResult {
	return &RegisterPrefetchResult{RequireComplexPassword: requireComplexPassword()}
}

func requireComplexPassword() bool {
	return conf.NotelmGlobal().Auth.RequireComplexPassword
}

func validatePasswordPolicy(plain string) error {
	if !requireComplexPassword() {
		return nil
	}

	return entity.ValidateComplexPassword(plain)
}

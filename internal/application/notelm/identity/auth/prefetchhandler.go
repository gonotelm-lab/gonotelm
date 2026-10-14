package auth

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// PrefetchHandler 表单前元数据：注册页与重置页共用同一份密码策略，从服务端配置取
type PrefetchHandler struct{}

func NewPrefetchHandler() *PrefetchHandler { return &PrefetchHandler{} }

type PrefetchResult struct {
	RequireComplexPassword bool
}

func (h *PrefetchHandler) Handle() *PrefetchResult {
	return &PrefetchResult{RequireComplexPassword: requireComplexPassword()}
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

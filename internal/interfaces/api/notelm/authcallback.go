package notelm

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// redirectCallbackError 回调失败一律 302 回前端：登录错误落登录页，绑定错误落账号设置页，return_to 优先
func (s *Server) redirectCallbackError(
	ctx context.Context,
	c *app.RequestContext,
	providerType entity.ProviderType,
	resp *authapp.CallbackHandleResult,
	err error,
) {
	var (
		auth   = conf.NotelmGlobal().Auth
		isBind = resp != nil && resp.Mode.IsBindMode()
		target = auth.LoginErrorURL
	)

	if isBind {
		target = auth.BindErrorURL
	}
	if resp != nil && resp.ReturnTo != "" && schema.IsSafeReturnTo(resp.ReturnTo, auth.ReturnToAllowOrigins) {
		target = resp.ReturnTo
	}

	query := schema.AuthCallbackErrorQuery{
		Reason:   schema.CallbackReason(err),
		Provider: providerType.String(),
	}
	if isBind {
		query.Settings = schema.CallbackSettingsAccount
	}

	targetURL, err := query.AppendTo(target)
	if err != nil {
		// 错误页地址和 return_to 都解析不了，只能回 JSON
		http.ErrResp(c, err)
		return
	}

	slog.WarnContext(ctx, "oauth callback failed",
		slog.String("provider", query.Provider),
		slog.String("reason", query.Reason),
		slog.Bool("bind", isBind),
		slog.Any("err", err),
	)

	c.Redirect(stdhttp.StatusFound, []byte(targetURL))
}

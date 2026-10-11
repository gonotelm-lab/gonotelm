package schema

import (
	"net/url"

	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	// CallbackReasonInternal 未识别的回调错误统一用它，不向前端泄露内部细节
	CallbackReasonInternal = "internal-error"
	// CallbackSettingsAccount 绑定失败时补的 settings 值，前端据此直接打开账号设置
	CallbackSettingsAccount = "account"

	CallbackReasonOAuthStateNotFound    = "oauth-state-not-found"
	CallbackReasonOAuthStateMismatch    = "oauth-state-mismatch"
	CallbackReasonProviderNotFound      = "provider-not-found"
	CallbackReasonIDPError              = "idp-error"
	CallbackReasonIDPExchangeError      = "idp-exchange-error"
	CallbackReasonIdpEmailUnavailable   = "idp-email-unavailable"
	CallbackReasonUserNotFound          = "user-not-found"
	CallbackReasonUserSessionNotFound   = "user-session-not-found"
	CallbackReasonUserBanned            = "user-banned"
	CallbackReasonIdentityNotFound      = "identity-not-found"
	CallbackReasonIdentityAlreadyLinked = "identity-already-linked"
	CallbackReasonProviderAlreadyBound  = "provider-already-bound"
	CallbackReasonEmailNotVerified      = "email-not-verified"
	CallbackReasonTooManyRequests       = "too-many-requests"
)

// CallbackReason 把 domain 错误翻成前端可读的 reason，横线格式。
func CallbackReason(err error) string {
	switch {
	case errors.Is(err, identerr.ErrOAuthStateNotFound):
		return CallbackReasonOAuthStateNotFound
	case errors.Is(err, identerr.ErrOAuthStateMismatch):
		return CallbackReasonOAuthStateMismatch
	case errors.Is(err, identerr.ErrProviderNotFound):
		return CallbackReasonProviderNotFound
	case errors.Is(err, identerr.ErrIDPError):
		return CallbackReasonIDPError
	case errors.Is(err, identerr.ErrIDPExchangeError):
		return CallbackReasonIDPExchangeError
	case errors.Is(err, identerr.ErrIdpEmailUnavailable):
		return CallbackReasonIdpEmailUnavailable
	case errors.Is(err, identerr.ErrUserNotFound):
		return CallbackReasonUserNotFound
	case errors.Is(err, identerr.ErrUserSessionNotFound):
		return CallbackReasonUserSessionNotFound
	case errors.Is(err, identerr.ErrUserBanned):
		return CallbackReasonUserBanned
	case errors.Is(err, identerr.ErrIdentityNotFound):
		return CallbackReasonIdentityNotFound
	case errors.Is(err, identerr.ErrIdentityAlreadyLinked):
		return CallbackReasonIdentityAlreadyLinked
	case errors.Is(err, identerr.ErrProviderAlreadyBound):
		return CallbackReasonProviderAlreadyBound
	case errors.Is(err, identerr.ErrEmailNotVerified):
		return CallbackReasonEmailNotVerified
	case errors.Is(err, identerr.ErrTooManyRequests):
		return CallbackReasonTooManyRequests
	default:
		return CallbackReasonInternal
	}
}

// AuthCallbackErrorQuery 回调失败跳转时附带的 query 参数。
type AuthCallbackErrorQuery struct {
	Reason   string
	Provider string
	// Settings 仅绑定失败时填 CallbackSettingsAccount
	Settings string
}

// Encode 转成 query 字符串（不含 '?'）。
func (q AuthCallbackErrorQuery) Encode() string {
	return q.values().Encode()
}

// AppendTo 把参数拼到 raw 上，保留 raw 已有的 query 与 fragment。
func (q AuthCallbackErrorQuery) AppendTo(raw string) (string, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	query := target.Query()
	for key, values := range q.values() {
		query[key] = values
	}
	target.RawQuery = query.Encode()

	return target.String(), nil
}

func (q AuthCallbackErrorQuery) values() url.Values {
	query := url.Values{}
	query.Set("reason", q.Reason)
	query.Set("provider", q.Provider)
	if q.Settings != "" {
		query.Set("settings", q.Settings)
	}

	return query
}

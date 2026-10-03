package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeProviderNotFound    = 106001
	CodeIDPError            = 106002
	CodeLoginStateNotFound  = 106003
	CodeLoginStateMismatch  = 106004
	CodeIDPExchangeError    = 106005
	CodeUserNotFound        = 106006
	CodeUserSessionNotFound = 106007
	CodeUserBanned          = 106008
	CodeInvalidNickname     = 106009
	CodeInvalidAvatar       = 106010
)

var (
	ErrProviderNotFound    = errors.ErrNoRecord.ErrCode(CodeProviderNotFound).Msg("provider not found")
	ErrIDPError            = errors.ErrInner.ErrCode(CodeIDPError).Msg("idp error")
	ErrLoginStateNotFound  = errors.ErrNoRecord.ErrCode(CodeLoginStateNotFound).Msg("login state not found")
	ErrLoginStateMismatch  = errors.ErrInner.ErrCode(CodeLoginStateMismatch).Msg("login state mismatch")
	ErrIDPExchangeError    = errors.ErrInner.ErrCode(CodeIDPExchangeError).Msg("idp exchange error")
	ErrUserNotFound        = errors.ErrNoRecord.ErrCode(CodeUserNotFound).Msg("user not found")
	ErrUserSessionNotFound = errors.ErrNoRecord.ErrCode(CodeUserSessionNotFound).Msg("user session not found")
	ErrUserBanned          = errors.ErrPermission.ErrCode(CodeUserBanned).Msg("user banned")

	ErrInvalidNickname = errors.ErrParams.ErrCode(CodeInvalidNickname).Msg("invalid nickname")
	ErrInvalidAvatar   = errors.ErrParams.ErrCode(CodeInvalidAvatar).Msg("invalid avatar")
)

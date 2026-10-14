package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeProviderNotFound    = 106001
	CodeIDPError            = 106002
	CodeOAuthStateNotFound  = 106003
	CodeOAuthStateMismatch  = 106004
	CodeIDPExchangeError    = 106005
	CodeUserNotFound        = 106006
	CodeUserSessionNotFound = 106007
	CodeUserBanned          = 106008
	CodeInvalidNickname     = 106009
	CodeInvalidAvatar       = 106010

	CodeInvalidCredentials           = 106011
	CodeEmailTaken                   = 106012
	CodeEmailInvalid                 = 106013
	CodeWeakPassword                 = 106014
	CodeIdpEmailUnavailable          = 106015
	CodeVerificationExpired          = 106016
	CodeVerificationAttemptsExceeded = 106017
	CodeVerificationCodeMismatch     = 106018
	CodeIdentityAlreadyLinked        = 106019
	CodeEmailNotVerified             = 106020
	CodeTooManyRequests              = 106021
	CodeIdentityNotFound             = 106022
	CodeProviderAlreadyBound         = 106023
	CodeLastLoginMethod              = 106024
	CodePasswordUnchanged            = 106025
	CodeMailSendFailed               = 106026
)

var (
	ErrProviderNotFound    = errors.ErrNoRecord.ErrCode(CodeProviderNotFound).Msg("provider not found")
	ErrIDPError            = errors.ErrInner.ErrCode(CodeIDPError).Msg("idp error")
	ErrOAuthStateNotFound  = errors.ErrNoRecord.ErrCode(CodeOAuthStateNotFound).Msg("oauth state not found")
	ErrOAuthStateMismatch  = errors.ErrInner.ErrCode(CodeOAuthStateMismatch).Msg("oauth state mismatch")
	ErrIDPExchangeError    = errors.ErrInner.ErrCode(CodeIDPExchangeError).Msg("idp exchange error")
	ErrUserNotFound        = errors.ErrNoRecord.ErrCode(CodeUserNotFound).Msg("user not found")
	ErrUserSessionNotFound = errors.ErrNoRecord.ErrCode(CodeUserSessionNotFound).Msg("user session not found")
	ErrUserBanned          = errors.ErrPermission.ErrCode(CodeUserBanned).Msg("user banned")

	ErrInvalidNickname = errors.ErrParams.ErrCode(CodeInvalidNickname).Msg("invalid nickname")
	ErrInvalidAvatar   = errors.ErrParams.ErrCode(CodeInvalidAvatar).Msg("invalid avatar")

	ErrInvalidCredentials           = errors.ErrUnauthorized.ErrCode(CodeInvalidCredentials).Msg("invalid email or password")
	ErrEmailTaken                   = errors.ErrParams.ErrCode(CodeEmailTaken).Msg("email already registered")
	ErrEmailInvalid                 = errors.ErrParams.ErrCode(CodeEmailInvalid).Msg("invalid email")
	ErrWeakPassword                 = errors.ErrParams.ErrCode(CodeWeakPassword).Msg("password does not meet the policy")
	ErrIdpEmailUnavailable          = errors.ErrPermission.ErrCode(CodeIdpEmailUnavailable).Msg("provider did not return a verified email")
	ErrVerificationExpired          = errors.ErrNoRecord.ErrCode(CodeVerificationExpired).Msg("verification code or link expired")
	ErrVerificationAttemptsExceeded = errors.ErrPermission.ErrCode(CodeVerificationAttemptsExceeded).Msg("too many verification attempts")
	ErrVerificationCodeMismatch     = errors.ErrParams.ErrCode(CodeVerificationCodeMismatch).Msg("verification code mismatch")
	ErrIdentityAlreadyLinked        = errors.ErrParams.ErrCode(CodeIdentityAlreadyLinked).Msg("identity already linked to another account")
	ErrEmailNotVerified             = errors.ErrPermission.ErrCode(CodeEmailNotVerified).Msg("email not verified")
	ErrTooManyRequests              = errors.ErrPermission.ErrCode(CodeTooManyRequests).Msg("too many requests")
	ErrIdentityNotFound             = errors.ErrNoRecord.ErrCode(CodeIdentityNotFound).Msg("sign-in identity not found")
	ErrProviderAlreadyBound         = errors.ErrParams.ErrCode(CodeProviderAlreadyBound).Msg("this account already bound another identity of the same provider")
	ErrLastLoginMethod              = errors.ErrParams.ErrCode(CodeLastLoginMethod).Msg("cannot unbind the last sign-in method, set a password first")
	ErrPasswordUnchanged            = errors.ErrParams.ErrCode(CodePasswordUnchanged).Msg("new password must differ from the current one")
	ErrMailSendFailed               = errors.ErrParams.ErrCode(CodeMailSendFailed).Msg("failed to send email, please retry later")
)

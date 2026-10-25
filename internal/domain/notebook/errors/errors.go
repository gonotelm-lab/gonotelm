package errors

import (
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	CodeNotebookNotFound     = 100001
	CodeInvalidName          = 100002
	CodeInvalidDescription   = 100003
	CodeInvalidOwnerId       = 100004
	CodeSourceCountExceeded  = 100005
	CodeNoSourcesForDesc     = 100006
	CodeInvalidMemberRole    = 100007
	CodeCannotShareWithOwner = 100008
	CodeMemberNotFound       = 100009
	CodeShareUserNotFound    = 100010
	CodeShareUserBanned      = 100011
	CodeInvalidNotebookId    = 100012
	CodeInvalidUserId        = 100013
	CodeInvalidVisibility    = 100014
	CodeCustomizeDescription = 100015
	CodeInvalidCover         = 100016
)

var (
	ErrNotebookNotFound = errors.ErrNotFound.ErrCode(CodeNotebookNotFound).Msg("notebook not found")

	ErrInvalidName         = errors.ErrParams.ErrCode(CodeInvalidName).Msg("invalid name")
	ErrInvalidDescription  = errors.ErrParams.ErrCode(CodeInvalidDescription).Msg("invalid description")
	ErrInvalidOwnerId      = errors.ErrParams.ErrCode(CodeInvalidOwnerId).Msg("invalid owner id")
	ErrSourceCountExceeded = errors.ErrParams.ErrCode(CodeSourceCountExceeded).Msg("source count exceeded")
	ErrNoSourcesForDesc    = errors.ErrParams.ErrCode(CodeNoSourcesForDesc).Msg("no ready sources available for description generation")

	ErrInvalidMemberRole    = errors.ErrParams.ErrCode(CodeInvalidMemberRole).Msg("invalid member role")
	ErrCannotShareWithOwner = errors.ErrParams.ErrCode(CodeCannotShareWithOwner).Msg("cannot share notebook with owner")
	ErrMemberNotFound       = errors.ErrNoRecord.ErrCode(CodeMemberNotFound).Msg("notebook member not found")
	ErrShareUserNotFound    = errors.ErrNoRecord.ErrCode(CodeShareUserNotFound).Msg("share user not found")
	ErrShareUserBanned      = errors.ErrPermission.ErrCode(CodeShareUserBanned).Msg("share user is banned")
	ErrInvalidNotebookId    = errors.ErrParams.ErrCode(CodeInvalidNotebookId).Msg("invalid notebook id")
	ErrInvalidUserId        = errors.ErrParams.ErrCode(CodeInvalidUserId).Msg("invalid user id")
	ErrInvalidVisibility    = errors.ErrParams.ErrCode(CodeInvalidVisibility).Msg("invalid visibility")

	ErrCustomizeDescription = errors.ErrParams.ErrCode(CodeCustomizeDescription).
				Msg("customize description is set")

	ErrInvalidCover = errors.ErrParams.ErrCode(CodeInvalidCover).Msg("invalid cover image")
)

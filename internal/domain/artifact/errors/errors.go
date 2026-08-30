package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeArtifactNotFound         = 102001
	CodeArtifactNotOwnedByUser   = 102002
	CodeCannotRetryCompleted     = 102003
	CodeCannotRetryNote          = 102004
	CodeCannotCancelInState      = 102005
	CodeCannotRetryInState       = 102006
	CodeCannotUpdateTitleInState = 102007
	CodeInvalidFlowTaskId        = 102008
	CodeInvalidNotebookId        = 102009
	CodeInvalidUserId            = 102010
	CodeInvalidKind              = 102011
	CodeInvalidPayload           = 102012
	CodePayloadKindMismatch      = 102013
)

var (
	ErrArtifactNotFound       = errors.ErrNoRecord.ErrCode(CodeArtifactNotFound).Msg("artifact not found")
	ErrArtifactNotOwnedByUser = errors.ErrPermission.ErrCode(CodeArtifactNotOwnedByUser).Msg("artifact not owned by user")

	ErrCannotRetryCompleted     = errors.ErrParams.ErrCode(CodeCannotRetryCompleted).Msg("cannot retry completed artifact")
	ErrCannotRetryNote          = errors.ErrParams.ErrCode(CodeCannotRetryNote).Msg("cannot retry note artifact, use generate instead")
	ErrCannotCancelInState      = errors.ErrParams.ErrCode(CodeCannotCancelInState).Msg("cannot cancel artifact in current state")
	ErrCannotRetryInState       = errors.ErrParams.ErrCode(CodeCannotRetryInState).Msg("cannot retry artifact in current state")
	ErrCannotUpdateTitleInState = errors.ErrParams.ErrCode(CodeCannotUpdateTitleInState).Msg("cannot update artifact title in current state")
	ErrInvalidFlowTaskId        = errors.ErrParams.ErrCode(CodeInvalidFlowTaskId).Msg("artifact has no flow task id")
	ErrInvalidNotebookId        = errors.ErrParams.ErrCode(CodeInvalidNotebookId).Msg("invalid notebook id")
	ErrInvalidUserId            = errors.ErrParams.ErrCode(CodeInvalidUserId).Msg("invalid user id")
	ErrInvalidKind              = errors.ErrParams.ErrCode(CodeInvalidKind).Msg("invalid artifact kind")
	ErrInvalidPayload           = errors.ErrParams.ErrCode(CodeInvalidPayload).Msg("invalid artifact payload")
	ErrPayloadKindMismatch      = errors.ErrParams.ErrCode(CodePayloadKindMismatch).Msg("artifact payload kind does not match artifact kind")
)

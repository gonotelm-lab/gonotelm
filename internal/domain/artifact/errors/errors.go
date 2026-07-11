package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrArtifactNotFound       = errors.ErrNoRecord.Msg("artifact not found")
	ErrArtifactNotOwnedByUser = errors.ErrPermission.Msg("artifact not owned by user")
	ErrCannotCancelInState    = errors.ErrParams.Msg("cannot cancel artifact in current state")
	ErrCannotRetryInState     = errors.ErrParams.Msg("cannot retry artifact in current state")
	ErrInvalidFlowTaskId      = errors.ErrParams.Msg("artifact has no flow task id")
)

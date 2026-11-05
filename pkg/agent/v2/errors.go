package agent

import pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrMaxRounds       = pkgerrors.ErrLLM.Msg("agent exceeded max rounds")
	ErrEmptyResponse   = pkgerrors.ErrLLM.Msg("model returned empty response")
	ErrContextOverflow = pkgerrors.ErrLLM.Msg("agent context exceeded budget")
	ErrBudgetExceeded  = pkgerrors.ErrLLM.Msg("agent run budget exceeded")
)

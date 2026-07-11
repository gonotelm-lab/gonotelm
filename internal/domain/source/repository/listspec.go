package repository

import (
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListSpec struct {
	Offset int
	Limit  int
}

func (s *ListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}

	return nil
}

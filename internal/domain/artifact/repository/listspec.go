package repository

import (
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
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

type ListByStatusSpec struct {
	Statuses []artifactentity.Status
	Limit    int
}

func (s *ListByStatusSpec) Validate() error {
	if len(s.Statuses) == 0 {
		return xerror.ErrParams.Msgf("statuses must not be empty")
	}
	if s.Limit <= 0 {
		return xerror.ErrParams.Msgf("invalid limit: %d", s.Limit)
	}
	return nil
}

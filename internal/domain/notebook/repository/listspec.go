package repository

import (
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListSpecOrder int

const (
	ListSpecOrderCreateTime ListSpecOrder = 0
	ListSpecOrderUpdateTime ListSpecOrder = 1
)

type ListSpec struct {
	Offset int
	Limit  int
	Order  ListSpecOrder
}

func (s *ListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}

	switch s.Order {
	case ListSpecOrderCreateTime, ListSpecOrderUpdateTime:
		return nil
	default:
		return xerror.ErrParams.Msgf("invalid order: %d", s.Order)
	}
}

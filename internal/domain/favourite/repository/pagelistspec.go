package repository

import (
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// PageListSpec 描述收藏夹 / 夹内列表的一页。
type PageListSpec struct {
	Limit  int
	Offset int
}

func (s *PageListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}
	return nil
}

package repository

import (
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// ListOrder 控制 notebook 列表的排序方式。
type ListOrder int

const (
	// ListOrderLastActive 按最后活跃时间倒序（updated_at DESC）。
	ListOrderLastActive ListOrder = 0
	// ListOrderCreateTime 按创建时间正序（uuidv7 id ASC，id 内嵌创建时间）。
	ListOrderCreateTime ListOrder = 1
)

// PageListSpec 描述 notebook 列表的一页。
type PageListSpec struct {
	Limit  int
	Offset int
	Order  ListOrder
}

func (s *PageListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}

	switch s.Order {
	case ListOrderLastActive, ListOrderCreateTime:
		return nil
	default:
		return xerror.ErrParams.Msgf("invalid order: %d", s.Order)
	}
}

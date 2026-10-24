package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/stretchr/testify/assert"
)

func TestAccessCanAcceptAsMember(t *testing.T) {
	ownerId := valobj.NewUid()
	access := &Access{Notebook: &Notebook{OwnerId: ownerId}}

	assert.False(t, access.CanAcceptAsMember(ownerId))
	assert.True(t, access.CanAcceptAsMember(valobj.NewUid()))
	assert.False(t, access.CanAcceptAsMember(valobj.Uid{}))
	assert.False(t, (*Access)(nil).CanAcceptAsMember(ownerId))
}

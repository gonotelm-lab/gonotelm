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

func TestNotebookCanPinBy(t *testing.T) {
	ownerId := valobj.NewUid()
	nb := &Notebook{OwnerId: ownerId}

	assert.True(t, nb.CanPinBy(ownerId, false))
	assert.True(t, nb.CanPinBy(valobj.NewUid(), true))
	assert.False(t, nb.CanPinBy(valobj.NewUid(), false))
	assert.False(t, (*Notebook)(nil).CanPinBy(ownerId, true))
}

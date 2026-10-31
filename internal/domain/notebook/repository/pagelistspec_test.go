package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageListSpec_Validate(t *testing.T) {
	assert.NoError(t, (&PageListSpec{Limit: 10, Order: ListOrderLastActive}).Validate())
	assert.NoError(t, (&PageListSpec{Limit: 1, Offset: 100, Order: ListOrderCreateTime}).Validate())
}

func TestPageListSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&PageListSpec{Limit: 0, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&PageListSpec{Limit: -1, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&PageListSpec{Limit: 10, Offset: -1, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&PageListSpec{Limit: 10, Order: ListOrder(2)}).Validate())
}

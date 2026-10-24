package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListSpec_Validate(t *testing.T) {
	assert.NoError(t, (&ListSpec{Limit: 10, Order: ListOrderLastActive}).Validate())
	assert.NoError(t, (&ListSpec{Limit: 1, Offset: 100, Order: ListOrderCreateTime}).Validate())
}

func TestListSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&ListSpec{Limit: 0, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&ListSpec{Limit: -1, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&ListSpec{Limit: 10, Offset: -1, Order: ListOrderLastActive}).Validate())
	assert.Error(t, (&ListSpec{Limit: 10, Order: ListOrder(2)}).Validate())
}

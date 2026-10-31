package repository

import (
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/stretchr/testify/assert"
)

func TestPageListSpec_Validate(t *testing.T) {
	assert.NoError(t, (&PageListSpec{Limit: 10, Offset: 0}).Validate())
	assert.NoError(t, (&PageListSpec{Limit: 1, Offset: 100}).Validate())
}

func TestPageListSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&PageListSpec{Limit: 0, Offset: 0}).Validate())
	assert.Error(t, (&PageListSpec{Limit: -1, Offset: 0}).Validate())
	assert.Error(t, (&PageListSpec{Limit: 10, Offset: -1}).Validate())
}

func TestPageListByStatusSpec_Validate(t *testing.T) {
	assert.NoError(t, (&PageListByStatusSpec{
		Statuses: []artifactentity.Status{artifactentity.StatusPending},
		Limit:    50,
	}).Validate())
}

func TestPageListByStatusSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&PageListByStatusSpec{Statuses: nil, Limit: 50}).Validate())
	assert.Error(t, (&PageListByStatusSpec{
		Statuses: []artifactentity.Status{artifactentity.StatusPending},
		Limit:    0,
	}).Validate())
}

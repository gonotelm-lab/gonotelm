package repository

import (
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/stretchr/testify/assert"
)

func TestListSpec_Validate(t *testing.T) {
	assert.NoError(t, (&ListSpec{Limit: 10, Offset: 0}).Validate())
	assert.NoError(t, (&ListSpec{Limit: 1, Offset: 100}).Validate())
}

func TestListSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&ListSpec{Limit: 0, Offset: 0}).Validate())
	assert.Error(t, (&ListSpec{Limit: -1, Offset: 0}).Validate())
	assert.Error(t, (&ListSpec{Limit: 10, Offset: -1}).Validate())
}

func TestListByStatusSpec_Validate(t *testing.T) {
	assert.NoError(t, (&ListByStatusSpec{
		Statuses: []artifactentity.Status{artifactentity.StatusPending},
		Limit:    50,
	}).Validate())
}

func TestListByStatusSpec_Validate_Errors(t *testing.T) {
	assert.Error(t, (&ListByStatusSpec{Statuses: nil, Limit: 50}).Validate())
	assert.Error(t, (&ListByStatusSpec{
		Statuses: []artifactentity.Status{artifactentity.StatusPending},
		Limit:    0,
	}).Validate())
}

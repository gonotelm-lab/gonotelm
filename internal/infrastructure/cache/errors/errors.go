package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrStreamNoData = errors.New("stream no data")
)

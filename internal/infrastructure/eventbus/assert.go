package eventbus

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func AssertEvent[T event.Event](evt event.Event) (T, error) {
	if evt == nil {
		var zero T
		return zero, errors.New("inner event is nil")
	}

	typed, ok := evt.(T)
	if !ok {
		var zero T
		return zero, errors.Errorf("inner event type mismatch: want %T, got %T", zero, evt)
	}

	return typed, nil
}

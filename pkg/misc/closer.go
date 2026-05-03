package misc

import "context"

type Closer interface {
	Close(ctx context.Context) error
}

type CloserFunc func(ctx context.Context) error

func (f CloserFunc) Close(ctx context.Context) error {
	return f(ctx)
}
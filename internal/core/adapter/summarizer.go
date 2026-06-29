package adapter

import "context"

type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
}

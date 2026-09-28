package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	"golang.org/x/sync/errgroup"
)

// parallel is a Step that runs its children concurrently. From the pipeline's
// point of view it is an ordinary sequential step: the whole group either
// succeeds or fails as one unit.
type parallel struct {
	name         string
	abortOnError bool
	steps        []Step
}

// NewParallelStep composes steps into a single Step that executes them
// concurrently.
//
// When abortOnError is true, the first child error cancels the remaining
// children, fails the group, and therefore aborts the pipeline. When
// abortOnError is false, child errors are logged and the group still succeeds
// once every child has finished.
func NewParallelStep(name string, abortOnError bool, steps ...Step) Step {
	return &parallel{
		name:         name,
		abortOnError: abortOnError,
		steps:        steps,
	}
}

func (s *parallel) Name() string { return s.name }

func (s *parallel) Execute(ctx context.Context, data *Data) error {
	if len(s.steps) == 0 {
		return nil
	}

	// With abortOnError, the group's context is cancelled as soon as a child
	// fails, which asks the remaining children to stop; Wait then returns that
	// first error.
	runCtx := ctx
	group := new(errgroup.Group)
	if s.abortOnError {
		group, runCtx = errgroup.WithContext(ctx)
	}

	for idx, step := range s.steps {
		if conditionalStep, ok := step.(ConditionalStep); ok {
			if !conditionalStep.ShouldExecute(runCtx, data) {
				continue
			}
		}

		group.Go(func() error {
			// A child panic is recovered by safe and surfaces as an error, so it
			// flows through the same abort/log decision as a returned error.
			err := safe.DoWithContext(runCtx, func(ctx context.Context) error {
				return step.Execute(ctx, data)
			})
			if err == nil {
				return nil
			}

			slog.ErrorContext(runCtx, fmt.Sprintf("parallel %s child %d (%s) failed", s.name, idx, step.Name()), slog.Any("err", err))

			if !s.abortOnError {
				return nil // log-only: a child failure must not fail the group
			}
			return err
		})
	}

	return group.Wait()
}

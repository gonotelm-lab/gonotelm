// Package initjob orchestrates one-shot business data initialization.
//
// In scope: task registration, ordering by ID, idempotent skipping (already
// succeeded / ShouldSkip), execution, persisting run and task records,
// strict/loose failure policy, exit semantics.
//
// Out of scope: transactions, timeouts, locking, task-level idempotency.
//
// Hard contract: Task.Run must be idempotent. The framework only guarantees
// that a task never runs again once its success is recorded. If the process
// dies between the data write and the success record, the task runs again on
// the next run and its own idempotency must absorb that.
package initjob

import "context"

// Task is one initialization task. To add one: define a struct, embed Base,
// implement ID and Run, then add a line to the registry.
type Task interface {
	// ID is the idempotency key: YYYYMMDD plus a 4-digit daily sequence,
	// 12 digits total, e.g. "202610050001". IDs are never reused: to correct
	// already-executed data, add a new task with a larger ID.
	ID() string

	// Description is a short human-readable summary of what the task does. It is
	// logged and stored on the task row, refreshed on every execution.
	// Base returns an empty string.
	Description() string

	// ShouldSkip skips the task and writes no record, so it is re-evaluated
	// on every run. Return false when the check itself fails: tasks are
	// idempotent, so running twice is the safe side.
	ShouldSkip(ctx context.Context) bool

	// Run executes the task. A non-nil error marks it failed.
	Run(ctx context.Context) error
}

// Base supplies the default ShouldSkip (never skip) and Description (empty)
// so that a new task only needs to implement ID and Run.
type Base struct{}

// ShouldSkip never skips.
func (Base) ShouldSkip(context.Context) bool { return false }

// Description is empty by default; override it to describe the task.
func (Base) Description() string { return "" }

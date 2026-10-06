// Package tasks holds the initialization tasks and the list that registers
// them. Adding a task is one task file plus one line in All.
package tasks

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/deps"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
)

// All returns every registered task in any order: the runner sorts them by ID,
// so registration order never decides execution order.
func All(d *deps.Instance) []pkginitjob.Task {
	return []pkginitjob.Task{
		// Add tasks here, e.g. NewSeedDefaultModels(d),
	}
}

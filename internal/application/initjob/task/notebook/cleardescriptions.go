package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task/idregistry"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
)

// clearDescriptionsTask 清空存量 notebook.description。
type clearDescriptionsTask struct {
	pkginitjob.Base
	store database.NotebookMaintenanceStore
}

func NewClearDescriptionsTask(store database.NotebookMaintenanceStore) pkginitjob.Task {
	return &clearDescriptionsTask{store: store}
}

var _ pkginitjob.Task = &clearDescriptionsTask{}

func (t *clearDescriptionsTask) ID() string { return idregistry.IdClearNotebookDescriptions }

func (t *clearDescriptionsTask) Description() string {
	return "Clear legacy notebook descriptions"
}

func (t *clearDescriptionsTask) ShouldSkip(ctx context.Context) bool {
	count, err := t.store.CountNonEmptyDescriptions(ctx)
	if err != nil {
		slog.WarnContext(ctx, "initjob count non-empty notebook descriptions failed, will run",
			"error", err)
		return false
	}
	return count == 0
}

func (t *clearDescriptionsTask) Run(ctx context.Context) error {
	err := t.store.ClearNonEmptyDescriptions(ctx, &schema.NotebookClearDescriptionsParams{
		UpdatedAt: valobj.NewTime().Value(),
	})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "initjob cleared legacy notebook descriptions")
	return nil
}

package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

// AIDescriptionRepository AI 摘要暂存
type AIDescriptionRepository interface {
	// miss时返回 ("", nil)。
	Get(ctx context.Context, notebookId valobj.Id, fingerprint string) (string, error)
	Save(ctx context.Context, notebookId valobj.Id, fingerprint, desc string) error
}

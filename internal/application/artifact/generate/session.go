package generate

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type SessionState struct {
	NotebookId valobj.Id
	SourceIds  []valobj.Id
	UserId     string
	Lang       string
}

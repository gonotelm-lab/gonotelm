package agent

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
)

type SessionState struct {
	userId string
	taskId valobj.Id

	chat     *chatentity.Chat
	notebook *notebookentity.Notebook
	sources  []*sourceentity.Source

	// sourceDoc
	sourceDocCitations []valobj.Id

	accumulatedId int64
	cancel        context.CancelFunc
	taskAborted   bool
}

func (s *SessionState) ChatId() valobj.Id {
	return s.chat.Id
}

func (s *SessionState) TaskId() valobj.Id {
	return s.taskId
}

func (s *SessionState) NotebookId() valobj.Id {
	return s.notebook.Id
}

func (s *SessionState) Sources() []*sourceentity.Source {
	return s.sources
}

func (s *SessionState) SourceDocCitations() []valobj.Id {
	return s.sourceDocCitations
}

func (s *SessionState) AccumulatedId() int64 {
	return s.accumulatedId
}

func (s *SessionState) Cancel() {
	s.cancel()
}

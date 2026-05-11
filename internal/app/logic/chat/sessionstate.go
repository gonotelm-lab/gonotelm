package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type chatSessionState struct {
	taskId   string
	chatId   uuid.UUID
	userId   string
	userLang string // TODO i18n

	sourceDocs []*model.SourceDoc // 本地对话选中的文档

	// transient state
	id          int64 // accumulated id
	cancel      context.CancelFunc
	taskAborted bool
}

func (s *chatSessionState) nextId() int64 {
	id := s.id
	s.id++
	return id
}

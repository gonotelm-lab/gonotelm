package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type chatSessionState struct {
	taskId string
	chatId uuid.UUID
	userId string

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

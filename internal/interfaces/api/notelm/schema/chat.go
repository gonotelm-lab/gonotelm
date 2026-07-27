package schema

import (
	"math"

	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/agent"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type ChatCreateMessageRequest struct {
	Id             uuid.UUID   `path:"id,required"`
	Prompt         string      `json:"prompt"`
	SourceIds      []uuid.UUID `json:"source_ids"`
	EnableThinking bool        `json:"enable_thinking"`
	Style          string      `json:"style"`
	AnswerLength   string      `json:"answer_length"`
}

func (r *ChatCreateMessageRequest) Validate() error {
	if r.Style == "" {
		r.Style = string(chatagent.ChatMessageStyleDefault)
	}

	if len(r.Prompt) == 0 {
		return errors.ErrParams.Msg("prompt is required")
	}

	if !chatagent.ChatMessageStyle(r.Style).IsValid() {
		return errors.ErrParams.Msgf("invalid chat style: %s", r.Style)
	}

	if r.AnswerLength == "" {
		r.AnswerLength = string(chatagent.ChatMessageAnswerLengthDefault)
	}

	if !chatagent.ChatMessageAnswerLength(r.AnswerLength).IsValid() {
		return errors.ErrParams.Msgf("invalid chat answer length: %s", r.AnswerLength)
	}

	return nil
}

type ChatCreateMessageResponse struct {
	MsgId  string `json:"msg_id"`
	TaskId string `json:"task_id"`
}

type ChatAbortStreamRequest struct {
	Id     uuid.UUID `path:"id,required"`
	TaskId string    `json:"task_id,required"`
}

type GetChatStreamRequest struct {
	Id           uuid.UUID `path:"id,required"` // chat id
	TaskId       string    `query:"task_id,required"`
	LastStreamId string    `query:"last_stream_id"`
}

type ListChatMessagesRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Cursor int64     `query:"cursor" validate:"min=0"`
	Limit  int       `query:"limit"  validate:"omitempty,min=1,max=100"`
}

const defaultChatMessagesListLimit = 20

func (r *ListChatMessagesRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultChatMessagesListLimit
	}
	if r.Cursor == 0 {
		r.Cursor = math.MaxInt64
	}

	return nil
}

type ListChatMessagesResponse struct {
	Messages   []*Message `json:"messages"`
	Limit      int        `json:"limit"`
	HasMore    bool       `json:"has_more"`
	NextCursor int64      `json:"next_cursor"`
}

type DeleteChatContextRequest struct {
	Id uuid.UUID `path:"id,required"`
}

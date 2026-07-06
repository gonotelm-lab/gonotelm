package chat

import "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/stream"

type PackedContent = stream.PackedContent

type HandleStreamResult = stream.HandleStreamResult

type EventType = stream.EventType

const (
	EventStart          = stream.EventStart
	EventContentStart   = stream.EventContentStart
	EventContentDelta   = stream.EventContentDelta
	EventContentEnd     = stream.EventContentEnd
	EventToolStart      = stream.EventToolStart
	EventToolDelta      = stream.EventToolDelta
	EventToolEnd        = stream.EventToolEnd
	EventReasoningStart = stream.EventReasoningStart
	EventReasoningDelta = stream.EventReasoningDelta
	EventReasoningEnd   = stream.EventReasoningEnd
	EventError          = stream.EventError
	EventDone           = stream.EventDone
)

type StreamErrorReason = stream.StreamErrorReason

const (
	StreamErrorReasonPanic             = stream.StreamErrorReasonPanic
	StreamErrorReasonReceiveError      = stream.StreamErrorReasonReceiveError
	StreamErrorReasonContextDone       = stream.StreamErrorReasonContextDone
	StreamErrorReasonConcatError       = stream.StreamErrorReasonConcatError
	StreamErrorReasonModelFinishReason = stream.StreamErrorReasonModelFinishReason
	StreamErrorReasonUnknown           = stream.StreamErrorReasonUnknown
)

type StreamError = stream.StreamError

type Callbacks = stream.Callbacks

var HandleStream = stream.HandleStream

var HandleStreamWithCallback = stream.HandleStreamWithCallback

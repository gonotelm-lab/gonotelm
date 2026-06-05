package errors

import "net/http"

// Errors that should return 200 status code
const (
	CodeInvalidParams = 1000
	CodeNoRecord      = 1001
	CodeEmbedErr      = 1002
	CodeLLMErr        = 1003
)

const (
	MsgInvalidParams = "INVALID_PARAMETERS"
	MsgNoRecord      = "NO_RECORD_FOUND"
	MsgEmbedErr      = "EMBEDDING_ERROR"
	MsgLLMErr        = "LLM_ERROR"
)

var (
	statusOk = http.StatusOK

	ErrParams   = NewInnerError(statusOk, CodeInvalidParams, MsgInvalidParams)
	ErrNoRecord = NewInnerError(statusOk, CodeNoRecord, MsgNoRecord)
	ErrEmbed    = NewInnerError(statusOk, CodeEmbedErr, MsgEmbedErr)
	ErrLLM      = NewInnerError(statusOk, CodeLLMErr, MsgLLMErr)
)

const (
	CodeUnauthorized = 2000
	CodePermission   = 2001
)

const (
	MsgUnauthorized = "UNAUTHORIZED"
	MsgPermission   = "PERMISSION_DENIED"
)

var (
	ErrUnauthorized = NewInnerError(http.StatusUnauthorized, CodeUnauthorized, MsgUnauthorized)
	ErrPermission   = NewInnerError(http.StatusForbidden, CodePermission, MsgPermission)
)

const (
	CodeDatabaseErr = -1 // database error
	CodeSerdeErr    = -2 // internal serialization/deserialization error
	CodeStorageErr  = -3 // storage error
	CodeMsgQueueErr = -4 // message queue error
	CodeCacheErr    = -5 // cache error

	CodeInnerErr   = -998
	CodeUnknownErr = -999
)

const (
	MsgDatabaseErr = "DATABASE_ERROR"
	MsgSerdeErr    = "SERDE_ERROR"
	MsgStorageErr  = "STORAGE_ERROR"
	MsgMsgQueueErr = "MESSAGE_QUEUE_ERROR"
	MsgCacheErr    = "CACHE_ERROR"
	MsgUnknownErr  = "UNKNOWN_ERROR"
	MsgInnerErr    = "INNER_ERROR"
)

// Internal errors that should return 5xx status code
var (
	statusInternal = http.StatusInternalServerError

	ErrDatabase = NewInnerError(statusInternal, CodeDatabaseErr, MsgDatabaseErr)
	ErrSerde    = NewInnerError(statusInternal, CodeSerdeErr, MsgSerdeErr)
	ErrStorage  = NewInnerError(statusInternal, CodeStorageErr, MsgStorageErr)
	ErrMsgQueue = NewInnerError(statusInternal, CodeMsgQueueErr, MsgMsgQueueErr)
	ErrCache    = NewInnerError(statusInternal, CodeCacheErr, MsgCacheErr)
	ErrInner    = NewInnerError(statusInternal, CodeInnerErr, MsgInnerErr)
	ErrUnknown  = NewInnerError(statusInternal, CodeUnknownErr, MsgUnknownErr)
)

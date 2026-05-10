package errors

import "net/http"

const (
	CodeInvalidParams = 1000
	CodeNoRecord      = 1001
	CodeEmbedErr      = 1002
)

const (
	CodeUnauthorized = 2000
	CodePermission   = 2001
)

const (
	CodeDatabaseErr = -1 // database error
	CodeSerdeErr    = -2 // internal serialization/deserialization error
	CodeStorageErr  = -3 // storage error
	CodeMsgQueueErr = -4 // message queue error
	CodeCacheErr    = -5 // cache error
	CodeUnknownErr  = -999
)

const (
	MsgInvalidParams = "invalid parameters"
	MsgNoRecord      = "no record found"
	MsgEmbedErr      = "embedding error"
)

// Errors that should return 200 status code
var (
	statusOk = http.StatusOK

	ErrParams   = NewInnerError(statusOk, CodeInvalidParams, MsgInvalidParams)
	ErrNoRecord = NewInnerError(statusOk, CodeNoRecord, MsgNoRecord)
	ErrEmbed    = NewInnerError(statusOk, CodeEmbedErr, MsgEmbedErr)
)

const (
	MsgUnauthorized = "unauthorized"
	MsgPermission   = "permission denied"
)

var (
	ErrUnauthorized = NewInnerError(http.StatusUnauthorized, CodeUnauthorized, MsgUnauthorized)
	ErrPermission   = NewInnerError(http.StatusForbidden, CodePermission, MsgPermission)
)

const (
	MsgDatabaseErr = "database error"
	MsgSerdeErr    = "serde error"
	MsgStorageErr  = "storage error"
	MsgMsgQueueErr = "message queue error"
	MsgCacheErr    = "cache error"
	MsgUnknownErr  = "unknown error"
)

// Internal errors that should return 5xx status code
var (
	statusInternal = http.StatusInternalServerError

	ErrDatabase = NewInnerError(statusInternal, CodeDatabaseErr, MsgDatabaseErr)
	ErrSerde    = NewInnerError(statusInternal, CodeSerdeErr, MsgSerdeErr)
	ErrStorage  = NewInnerError(statusInternal, CodeStorageErr, MsgStorageErr)
	ErrMsgQueue = NewInnerError(statusInternal, CodeMsgQueueErr, MsgMsgQueueErr)
	ErrCache    = NewInnerError(statusInternal, CodeCacheErr, MsgCacheErr)
	ErrUnknown  = NewInnerError(statusInternal, CodeUnknownErr, MsgUnknownErr)
)

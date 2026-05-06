package errors

import "net/http"

const (
	CodeInvalidParams = 1000
	CodeNoRecord      = 1001
	CodeEmbedErr      = 1002
)

const (
	CodeUnauthorized = 2000
)

const (
	CodeDatabaseErr = -1 // database error
	CodeSerdeErr    = -2 // internal serialization/deserialization error
	CodeStorageErr  = -3 // storage error
	CodeMsgQueueErr = -4 // message queue error
	CodeCacheErr    = -5 // cache error
	CodeUnknownErr  = -999
)

// Errors that should return 200 status code
var (
	statusOk = http.StatusOK

	ErrParams   = NewInnerError(statusOk, CodeInvalidParams, "invalid parameters")
	ErrNoRecord = NewInnerError(statusOk, CodeNoRecord, "no record found")
	ErrEmbed    = NewInnerError(statusOk, CodeEmbedErr, "embedding error")
)

// TODO Errors that should return 4xx status code, like unauthorized, etc.
var (
	ErrUnauthorized = NewInnerError(http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
)

// Internal errors that should return 5xx status code
var (
	statusInternal = http.StatusInternalServerError

	ErrDatabase = NewInnerError(statusInternal, CodeDatabaseErr, "database error")
	ErrSerde    = NewInnerError(statusInternal, CodeSerdeErr, "serde error")
	ErrStorage  = NewInnerError(statusInternal, CodeStorageErr, "storage error")
	ErrMsgQueue = NewInnerError(statusInternal, CodeMsgQueueErr, "message queue error")
	ErrCache    = NewInnerError(statusInternal, CodeCacheErr, "cache error")
)

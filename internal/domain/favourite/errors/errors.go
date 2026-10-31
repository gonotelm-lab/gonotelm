package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeFavouriteLimitExceeded         = 108001
	CodeFavouriteNotebookLimitExceeded = 108002
	CodeFavouriteNotFound              = 108003
	CodeInvalidFavouriteName           = 108004
	CodeInvalidNotebookId              = 108005
	CodeInvalidUserId                  = 108006
	CodeInvalidFavouriteId             = 108007
	CodeFavouriteNotebookNotFound      = 108008
)

var (
	ErrFavouriteLimitExceeded         = errors.ErrParams.ErrCode(CodeFavouriteLimitExceeded).Msg("favourite limit exceeded")
	ErrFavouriteNotebookLimitExceeded = errors.ErrParams.ErrCode(CodeFavouriteNotebookLimitExceeded).Msg("favourite notebook limit exceeded")
	ErrFavouriteNotFound              = errors.ErrNoRecord.ErrCode(CodeFavouriteNotFound).Msg("favourite not found")
	ErrInvalidFavouriteName           = errors.ErrParams.ErrCode(CodeInvalidFavouriteName).Msg("invalid favourite name")
	ErrInvalidNotebookId              = errors.ErrParams.ErrCode(CodeInvalidNotebookId).Msg("invalid notebook id")
	ErrInvalidUserId                  = errors.ErrParams.ErrCode(CodeInvalidUserId).Msg("invalid user id")
	ErrInvalidFavouriteId             = errors.ErrParams.ErrCode(CodeInvalidFavouriteId).Msg("invalid favourite id")
	ErrFavouriteNotebookNotFound      = errors.ErrNoRecord.ErrCode(CodeFavouriteNotebookNotFound).Msg("favourite notebook not found")
)

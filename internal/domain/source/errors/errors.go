package source

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeSourceNotFound               = 103001
	CodeSourceContentTooLarge        = 103002
	CodeSourceUnsupportedContentType = 103003
	CodeSourceInvalidURL             = 103004
	CodeSourceContentCorrupted       = 103005
)

var (
	ErrSourceNotFound = errors.ErrNoRecord.ErrCode(CodeSourceNotFound).Msg("source not found")

	ErrSourceContentTooLarge        = errors.ErrParams.ErrCode(CodeSourceContentTooLarge).Msg("source content too large")
	ErrSourceUnsupportedContentType = errors.ErrParams.ErrCode(CodeSourceUnsupportedContentType).Msg("unsupported content type")
	ErrSourceInvalidURL             = errors.ErrParams.ErrCode(CodeSourceInvalidURL).Msg("invalid source url")
	ErrSourceContentCorrupted       = errors.ErrParams.ErrCode(CodeSourceContentCorrupted).Msg("source content corrupt")
)

func IsSourceInvalidError(err error) bool {
	err = errors.Cause(err)

	return errors.Is(err, ErrSourceContentTooLarge) ||
		errors.Is(err, ErrSourceUnsupportedContentType) ||
		errors.Is(err, ErrSourceInvalidURL) ||
		errors.Is(err, ErrSourceContentCorrupted)
}

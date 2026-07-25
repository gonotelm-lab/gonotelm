package string

import (
	"unsafe"
)

func AsBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func FromBytes(s []byte) string {
	return unsafe.String(unsafe.SliceData(s), len(s))
}

// StripJSONPrefix removes any text before the first '{' or '[' in s.
// Returns the trimmed content from the first JSON token onward, or the
// original string if neither is found.
func StripJSONPrefix(s string) string {
	idx := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '[':
			idx = i
			goto found
		}
	}
found:
	if idx > 0 {
		s = s[idx:]
	}
	return s
}

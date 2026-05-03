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

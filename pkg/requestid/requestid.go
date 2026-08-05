package requestid

import (
	"encoding/hex"

	"github.com/google/uuid"
)

// HeaderKey is the HTTP header name of the request id.
const HeaderKey = "X-Request-Id"

// ID is the request id, 16 bytes.
type ID [16]byte

var zeroID ID

// Gen returns a random request id.
func Gen() ID {
	return ID(uuid.New())
}

// ParseString parses a request id from its string representation
// (32 hex chars with or without dashes).
func ParseString(s string) (ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ID{}, err
	}

	return ID(u), nil
}

// String returns the 32 hex chars representation of the request id.
func (id ID) String() string {
	if id.IsZero() {
		return ""
	}

	var buf [32]byte
	hex.Encode(buf[:], id[:])
	return string(buf[:])
}

// Bytes returns the underlying bytes of the request id.
func (id ID) Bytes() []byte {
	return id[:]
}

// IsZero returns true if the request id is all zeros.
func (id ID) IsZero() bool {
	return id == zeroID
}

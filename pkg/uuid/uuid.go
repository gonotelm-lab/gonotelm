package uuid

import (
	"database/sql/driver"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"time"

	googl "github.com/google/uuid"
)

var (
	zerou = UUID{googl.Nil}
	maxu  = UUID{googl.Max}
)

type UUID struct {
	googl.UUID
}

var (
	_ driver.Valuer          = UUID{}
	_ encoding.TextMarshaler = UUID{}
)

func EmptyUUID() UUID {
	return zerou
}

func MaxUUID() UUID {
	return maxu
}

func (u UUID) Duplicate() UUID {
	dst := [16]byte{}
	copy(dst[:], u.UUID[:])
	return UUID{dst}
}

func (u UUID) Time() time.Time {
	t := u.UUID.Time()
	sec, nesc := t.UnixTime() // unix time with second and nanosec
	return time.Unix(sec, nesc)
}

func (u UUID) UnixSec() int64 {
	return u.Time().Unix()
}

func (u UUID) UnixMill() int64 {
	return u.Time().UnixMilli()
}

func ParseString(s string) (UUID, error) {
	u, err := googl.Parse(s)
	if err != nil {
		return EmptyUUID(), err
	}
	return UUID{u}, nil
}

func NewV7() UUID {
	return UUID{googl.Must(googl.NewV7())}
}

func NewV4() UUID {
	return UUID{googl.New()}
}

func FromBytes(b []byte) (UUID, error) {
	raw, err := googl.ParseBytes(b)
	if err != nil {
		return EmptyUUID(), err
	}

	return UUID{raw}, nil
}

// compare u to o, return -1 if u < o, 0 if u == o, 1 if u > o
func (u UUID) Compare(o UUID) int {
	for idx := range 16 {
		if u.UUID[idx] < o.UUID[idx] {
			return -1
		} else if u.UUID[idx] > o.UUID[idx] {
			return 1
		}
	}
	return 0
}

func (u UUID) GreaterThan(o UUID) bool {
	return u.Compare(o) > 0
}

func (u UUID) NotEqualsTo(o UUID) bool {
	return u.Compare(o) != 0
}

func (u UUID) EqualsTo(o UUID) bool {
	return u.Compare(o) == 0
}

func (u UUID) LessThan(o UUID) bool {
	return u.Compare(o) < 0
}

func (u UUID) IsZero() bool {
	return u.EqualsTo(zerou)
}

func (u UUID) IsMax() bool {
	return u.EqualsTo(maxu)
}

// Return the string representation of the uuid
func (u UUID) String() string {
	if !u.IsZero() {
		var buf [32]byte
		encodeHex(buf[:], u.UUID)
		return string(buf[:])
	}
	return ""
}

func (u UUID) Bytes() []byte {
	return u.UUID[:]
}

func (u UUID) Base64() string {
	return base64.RawStdEncoding.EncodeToString(u.Bytes())
}

// Encode the uuid to a 32 byte slice
func encodeHex(dst []byte, uuid googl.UUID) {
	hex.Encode(dst, uuid[:4])
	hex.Encode(dst[8:12], uuid[4:6])
	hex.Encode(dst[12:16], uuid[6:8])
	hex.Encode(dst[16:20], uuid[8:10])
	hex.Encode(dst[20:], uuid[10:])
}

// Implements the encoding.TextMarshaler interface
func (u UUID) MarshalText() ([]byte, error) {
	var js [32]byte
	encodeHex(js[:], u.UUID)
	return js[:], nil
}

// Implement the driver.Valuer interface
func (u UUID) Value() (driver.Value, error) {
	return u.UUID[:], nil
}

package ulid

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"time"

	v2 "github.com/oklog/ulid/v2"
)

var (
	zeroULID = ULID{v2.Zero}

	// lowerEncoding is the lowercase Crockford base32 alphabet.
	// ULID strings only ever contain these 26 characters.
	lowerEncoding = "0123456789abcdefghjkmnpqrstvwxyz"
)

type ULID struct {
	v2.ULID
}

type ULIDArray []ULID

var (
	_ driver.Valuer            = ULID{}
	_ sql.Scanner              = (*ULID)(nil)
	_ encoding.TextMarshaler   = ULID{}
	_ encoding.TextUnmarshaler = (*ULID)(nil)
)

func EmptyULID() ULID {
	return zeroULID
}

func (u ULID) Duplicate() ULID {
	dst := [16]byte{}
	copy(dst[:], u.ULID[:])
	return ULID{dst}
}

func (u ULID) Time() uint64 {
	return u.ULID.Time()
}

func (u ULID) Timestamp() time.Time {
	return u.ULID.Timestamp()
}

func (u ULID) UnixMilli() int64 {
	return int64(u.ULID.Time())
}

func ParseString(s string) (ULID, error) {
	id, err := v2.ParseStrict(s)
	if err != nil {
		return EmptyULID(), err
	}
	return ULID{id}, nil
}

func MustParseString(s string) ULID {
	return ULID{v2.MustParseStrict(s)}
}

func New() ULID {
	return ULID{v2.Make()}
}

func FromBytes(b []byte) (ULID, error) {
	var id v2.ULID
	err := id.UnmarshalBinary(b)
	if err != nil {
		return EmptyULID(), err
	}
	return ULID{id}, nil
}

func (u ULID) Compare(o ULID) int {
	return u.ULID.Compare(o.ULID)
}

func (u ULID) GreaterThan(o ULID) bool {
	return u.Compare(o) > 0
}

func (u ULID) NotEqualsTo(o ULID) bool {
	return u.Compare(o) != 0
}

func (u ULID) EqualsTo(o ULID) bool {
	return u.Compare(o) == 0
}

func (u ULID) LessThan(o ULID) bool {
	return u.Compare(o) < 0
}

func (u ULID) IsZero() bool {
	return u.EqualsTo(zeroULID)
}

func (u ULID) String() string {
	id := u.ULID

	// Encode directly to lowercase, avoiding the uppercase encode +
	// strings.ToLower roundtrip (one allocation instead of three).
	var buf [26]byte

	// 10 byte timestamp (48 bits)
	buf[0] = lowerEncoding[(id[0]&224)>>5]
	buf[1] = lowerEncoding[id[0]&31]
	buf[2] = lowerEncoding[(id[1]&248)>>3]
	buf[3] = lowerEncoding[((id[1]&7)<<2)|((id[2]&192)>>6)]
	buf[4] = lowerEncoding[(id[2]&62)>>1]
	buf[5] = lowerEncoding[((id[2]&1)<<4)|((id[3]&240)>>4)]
	buf[6] = lowerEncoding[((id[3]&15)<<1)|((id[4]&128)>>7)]
	buf[7] = lowerEncoding[(id[4]&124)>>2]
	buf[8] = lowerEncoding[((id[4]&3)<<3)|((id[5]&224)>>5)]
	buf[9] = lowerEncoding[id[5]&31]

	// 16 bytes of entropy (80 bits)
	buf[10] = lowerEncoding[(id[6]&248)>>3]
	buf[11] = lowerEncoding[((id[6]&7)<<2)|((id[7]&192)>>6)]
	buf[12] = lowerEncoding[(id[7]&62)>>1]
	buf[13] = lowerEncoding[((id[7]&1)<<4)|((id[8]&240)>>4)]
	buf[14] = lowerEncoding[((id[8]&15)<<1)|((id[9]&128)>>7)]
	buf[15] = lowerEncoding[(id[9]&124)>>2]
	buf[16] = lowerEncoding[((id[9]&3)<<3)|((id[10]&224)>>5)]
	buf[17] = lowerEncoding[id[10]&31]
	buf[18] = lowerEncoding[(id[11]&248)>>3]
	buf[19] = lowerEncoding[((id[11]&7)<<2)|((id[12]&192)>>6)]
	buf[20] = lowerEncoding[(id[12]&62)>>1]
	buf[21] = lowerEncoding[((id[12]&1)<<4)|((id[13]&240)>>4)]
	buf[22] = lowerEncoding[((id[13]&15)<<1)|((id[14]&128)>>7)]
	buf[23] = lowerEncoding[(id[14]&124)>>2]
	buf[24] = lowerEncoding[((id[14]&3)<<3)|((id[15]&224)>>5)]
	buf[25] = lowerEncoding[id[15]&31]

	return string(buf[:])
}

func (u ULID) Bytes() []byte {
	return u.ULID.Bytes()
}

func (u ULID) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}

func (u *ULID) UnmarshalText(data []byte) error {
	parsed, err := ParseString(string(data))
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

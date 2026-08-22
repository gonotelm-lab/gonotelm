package valobj

import "github.com/gonotelm-lab/gonotelm/pkg/ulid"

// user id representation
type Uid = ulid.ULID

func NewUid() Uid {
	return ulid.New()
}

func NewUidFromString(s string) (Uid, error) {
	return ulid.ParseString(s)
}

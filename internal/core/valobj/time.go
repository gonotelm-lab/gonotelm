package valobj

import "time"

type Time struct {
	unixMs int64
}

func NewTime() Time {
	return Time{
		unixMs: time.Now().UnixMilli(),
	}
}

func (t Time) Value() int64 {
	return t.unixMs
}

func (t Time) Time() time.Time {
	return time.UnixMilli(t.unixMs)
}

func NewTimeFrom(unixMs int64) Time {
	return Time{
		unixMs: unixMs,
	}
}

func NewTimeFromId(id Id) Time {
	return NewTimeFrom(id.Time().UnixMilli())
}
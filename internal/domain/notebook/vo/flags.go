package vo

// Flags notebook 位标记。
type Flags int32

const (
	FlagCustomizeDescription Flags = 1 << iota
)

func (f Flags) Int32() int32 {
	return int32(f)
}

func NewFlags(v int32) Flags {
	return Flags(v)
}

func (f Flags) CustomizeDescription() bool {
	return f&FlagCustomizeDescription != 0
}

func (f Flags) WithCustomizeDescription(set bool) Flags {
	if set {
		return f | FlagCustomizeDescription
	}
	return f &^ FlagCustomizeDescription
}

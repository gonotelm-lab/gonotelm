package bitmap

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// import "encoding/binary"

type Bitmap struct {
	bitCount uint32

	// bits layout:
	// High                                               Low
	// [-------------------------------|--------------------]
	// |<N------bits content-------1-0>|<-bit count varint->|
	bits []byte
}

func alignTo8(bitCount uint32) int {
	// ceil(bitCount / 8)
	return int((bitCount + 7) >> 3)
}

func New(bitCount uint32) *Bitmap {
	// 对齐到8bit
	ab := alignTo8(bitCount)
	cb := binary.AppendUvarint(nil, uint64(bitCount))
	bits := make([]byte, ab+len(cb))
	copy(bits[ab:], cb)

	return &Bitmap{
		bitCount: bitCount,
		bits:     bits,
	}
}

func (b *Bitmap) Set(index uint32) {
	bitIndex := index % 8
	byteIndex := index / 8
	b.bits[byteIndex] |= 1 << bitIndex
}

func (b *Bitmap) Unset(index uint32) {
	bitIndex := index % 8
	byteIndex := index / 8
	b.bits[byteIndex] &= ^(1 << bitIndex)
}

func (b *Bitmap) Get(index uint32) bool {
	bitIndex := index % 8
	byteIndex := index / 8
	return b.bits[byteIndex]&(1<<bitIndex) != 0
}

// 获取所有bit为set的索引位置
func (b *Bitmap) GetAllSet() []uint32 {
	out := make([]uint32, 0, b.bitCount)
	for i := uint32(0); i < b.bitCount; i++ {
		if b.Get(i) {
			out = append(out, i)
		}
	}
	return out
}

func (b *Bitmap) String() string {
	return hex.EncodeToString(b.bits)
}

func NewFrom(s string) (*Bitmap, error) {
	bits, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	// 编码布局是 [bitmap-content][uvarint(bitCount)]，count 在尾部。
	// 反解时尝试所有可能的 varint 长度，找到满足布局的唯一合法解。
	maxCountBytes := binary.MaxVarintLen32
	if len(bits) < maxCountBytes {
		maxCountBytes = len(bits)
	}
	for countBytes := 1; countBytes <= maxCountBytes; countBytes++ {
		start := len(bits) - countBytes
		count, n := binary.Uvarint(bits[start:])
		if n != countBytes {
			continue
		}
		if count > uint64(^uint32(0)) {
			continue
		}
		if alignTo8(uint32(count))+countBytes != len(bits) {
			continue
		}

		return &Bitmap{
			bitCount: uint32(count),
			bits:     bits,
		}, nil
	}

	return nil, errors.New("invalid count")
}

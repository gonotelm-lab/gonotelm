package bitmap

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"slices"
	"testing"
)

func TestAlignTo8(t *testing.T) {
	testCases := []struct {
		name     string
		bitCount uint32
		want     int
	}{
		{name: "zero", bitCount: 0, want: 0},
		{name: "one", bitCount: 1, want: 1},
		{name: "exactly_one_byte", bitCount: 8, want: 1},
		{name: "cross_byte", bitCount: 9, want: 2},
		{name: "large", bitCount: 145, want: 19},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := alignTo8(tc.bitCount)
			if got != tc.want {
				t.Fatalf("alignTo8(%d) = %d, want %d", tc.bitCount, got, tc.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	bitCount := uint32(145)
	bm := New(bitCount)

	if bm.bitCount != bitCount {
		t.Fatalf("bitmap bitCount = %d, want %d", bm.bitCount, bitCount)
	}

	alignedBytes := alignTo8(bitCount)
	countBytes := binary.AppendUvarint(nil, uint64(bitCount))
	wantLen := alignedBytes + len(countBytes)
	if len(bm.bits) != wantLen {
		t.Fatalf("bitmap bits len = %d, want %d", len(bm.bits), wantLen)
	}

	if !bytes.Equal(bm.bits[alignedBytes:], countBytes) {
		t.Fatalf("bitmap count bytes = %v, want %v", bm.bits[alignedBytes:], countBytes)
	}

	for i := 0; i < alignedBytes; i++ {
		if bm.bits[i] != 0 {
			t.Fatalf("bitmap content byte[%d] = %d, want 0", i, bm.bits[i])
		}
	}
}

func TestSetAndUnset(t *testing.T) {
	bm := New(16)
	bm.Set(0)
	bm.Set(9)
	bm.Set(15)

	if bm.bits[0] != 0b00000001 {
		t.Fatalf("byte[0] after set = %08b, want %08b", bm.bits[0], byte(0b00000001))
	}
	if bm.bits[1] != 0b10000010 {
		t.Fatalf("byte[1] after set = %08b, want %08b", bm.bits[1], byte(0b10000010))
	}

	bm.Unset(9)
	if bm.bits[1] != 0b10000000 {
		t.Fatalf("byte[1] after clear = %08b, want %08b", bm.bits[1], byte(0b10000000))
	}
}

func TestGet(t *testing.T) {
	bm := New(24)
	bm.Set(0)
	bm.Set(9)
	bm.Set(23)

	testCases := []struct {
		name  string
		index uint32
		want  bool
	}{
		{name: "first_bit_set", index: 0, want: true},
		{name: "cross_byte_bit_set", index: 9, want: true},
		{name: "last_bit_set", index: 23, want: true},
		{name: "middle_bit_unset", index: 10, want: false},
		{name: "near_end_unset", index: 22, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := bm.Get(tc.index)
			if got != tc.want {
				t.Fatalf("Get(%d) = %v, want %v", tc.index, got, tc.want)
			}
		})
	}
}

func TestGetAllSet(t *testing.T) {
	t.Run("return_sorted_set_indices", func(t *testing.T) {
		bm := New(24)
		bm.Set(5)
		bm.Set(1)
		bm.Set(17)
		bm.Set(9)
		bm.Unset(9)
		bm.Set(23)

		got := bm.GetAllSet()
		want := []uint32{1, 5, 17, 23}
		if !slices.Equal(got, want) {
			t.Fatalf("GetAllSet() = %v, want %v", got, want)
		}
	})

	t.Run("empty_bitmap", func(t *testing.T) {
		bm := New(16)
		got := bm.GetAllSet()
		if len(got) != 0 {
			t.Fatalf("GetAllSet() len = %d, want 0", len(got))
		}
	})
}

func TestStringRoundTrip(t *testing.T) {
	origin := New(145)
	origin.Set(0)
	origin.Set(10)
	origin.Set(64)
	origin.Set(144)

	encoded := origin.String()
	t.Log(encoded)
	restored, err := NewFrom(encoded)
	if err != nil {
		t.Fatalf("NewFrom() unexpected error: %v", err)
	}

	if restored.bitCount != origin.bitCount {
		t.Fatalf("restored bitCount = %d, want %d", restored.bitCount, origin.bitCount)
	}
	if !bytes.Equal(restored.bits, origin.bits) {
		t.Fatalf("restored bits = %v, want %v", restored.bits, origin.bits)
	}
	if restored.String() != encoded {
		t.Fatalf("restored String() = %s, want %s", restored.String(), encoded)
	}
}

func TestNewFromErrors(t *testing.T) {
	t.Run("invalid_hex", func(t *testing.T) {
		if _, err := NewFrom("not-hex!"); err == nil {
			t.Fatalf("NewFrom should return error for invalid hex")
		}
	})

	t.Run("invalid_count_layout", func(t *testing.T) {
		encoded := hex.EncodeToString([]byte{0, 0})
		if _, err := NewFrom(encoded); err == nil {
			t.Fatalf("NewFrom should return error for invalid count layout")
		}
	})
}

package vo

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFlagsCustomizeDescription(t *testing.T) {
	Convey("Flags custom description bit", t, func() {
		Convey("default is unset", func() {
			So(NewFlags(0).CustomizeDescription(), ShouldBeFalse)
		})

		Convey("WithCustomizeDescription sets and clears the bit", func() {
			on := NewFlags(0).WithCustomizeDescription(true)
			So(on.CustomizeDescription(), ShouldBeTrue)
			So(on.Int32(), ShouldEqual, int32(FlagCustomizeDescription))

			off := on.WithCustomizeDescription(false)
			So(off.CustomizeDescription(), ShouldBeFalse)
			So(off.Int32(), ShouldEqual, int32(0))
		})

		Convey("other bits are preserved when toggling custom description", func() {
			const other Flags = 1 << 3
			f := (NewFlags(0) | other).WithCustomizeDescription(true)
			So(f&other, ShouldEqual, other)
			So(f.CustomizeDescription(), ShouldBeTrue)

			f = f.WithCustomizeDescription(false)
			So(f&other, ShouldEqual, other)
			So(f.CustomizeDescription(), ShouldBeFalse)
		})
	})
}

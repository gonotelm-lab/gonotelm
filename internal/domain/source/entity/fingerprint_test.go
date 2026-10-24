package entity

import (
	"testing"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSourceFingerprint(t *testing.T) {
	Convey("Source.Fingerprint", t, func() {
		Convey("empty title and abstract yields empty fingerprint", func() {
			s := &Source{Base: coreentity.NewBase(), Title: "  ", Abstract: ""}
			So(s.Fingerprint(), ShouldEqual, "")
		})

		Convey("includes id title abstract", func() {
			s := &Source{
				Base:     coreentity.Base{Id: valobj.NewId()},
				Title:    " t ",
				Abstract: " a ",
			}
			So(s.Fingerprint(), ShouldEqual, s.Id.String()+"\nt\na")
		})
	})
}

func TestNewFingerprint(t *testing.T) {
	Convey("NewFingerprint", t, func() {
		readyA := &Source{
			Base:     coreentity.Base{Id: uuid.MustParseString("01900000-0000-7000-8000-00000000000a")},
			Status:   vo.SourceStatusReady,
			Title:    "ta",
			Abstract: "aa",
		}
		readyB := &Source{
			Base:     coreentity.Base{Id: uuid.MustParseString("01900000-0000-7000-8000-00000000000b")},
			Status:   vo.SourceStatusReady,
			Title:    "tb",
			Abstract: "ab",
		}
		notReady := &Source{
			Base:     coreentity.Base{Id: valobj.NewId()},
			Status:   vo.SourceStatusPreparing,
			Title:    "x",
			Abstract: "y",
		}

		Convey("order independent", func() {
			a := NewFingerprint([]*Source{readyB, readyA})
			b := NewFingerprint([]*Source{readyA, readyB})
			So(a, ShouldEqual, b)
			So(a, ShouldNotEqual, "")
		})

		Convey("skips non-ready sources", func() {
			with := NewFingerprint([]*Source{readyA, notReady})
			without := NewFingerprint([]*Source{readyA})
			So(with, ShouldEqual, without)
		})
	})
}

package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNotebookCustomizeDescription(t *testing.T) {
	Convey("Notebook custom description", t, func() {
		ownerId := valobj.NewUid()
		n, err := NewNotebook("nb", ownerId)
		So(err, ShouldBeNil)
		So(n.Description, ShouldEqual, "")
		So(n.CustomizeDescription(), ShouldBeFalse)

		Convey("SetCustomizeDescription requires non-empty text", func() {
			err := n.SetCustomizeDescription("   ")
			So(errors.Is(err, notebookerrors.ErrInvalidDescription), ShouldBeTrue)
			So(n.CustomizeDescription(), ShouldBeFalse)
		})

		Convey("SetCustomizeDescription sets flag and text", func() {
			err := n.SetCustomizeDescription("  my summary  ")
			So(err, ShouldBeNil)
			So(n.Description, ShouldEqual, "my summary")
			So(n.CustomizeDescription(), ShouldBeTrue)
		})

		Convey("ClearCustomizeDescription clears flag and text", func() {
			So(n.SetCustomizeDescription("keep"), ShouldBeNil)
			n.ClearCustomizeDescription()
			So(n.Description, ShouldEqual, "")
			So(n.CustomizeDescription(), ShouldBeFalse)
		})
	})
}

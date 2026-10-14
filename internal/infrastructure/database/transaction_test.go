package database

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWithTxAndTxFromContext(t *testing.T) {
	Convey("WithTx stores a handle in ctx and TxFromContext reads it back", t, func() {
		ctx := WithTx(context.Background(), "tx-handle")

		tx, ok := TxFromContext(ctx)
		So(ok, ShouldBeTrue)
		So(tx, ShouldEqual, "tx-handle")
	})
}

func TestTxFromContextWithoutHandle(t *testing.T) {
	Convey("TxFromContext reports false when ctx carries no handle", t, func() {
		tx, ok := TxFromContext(context.Background())
		So(ok, ShouldBeFalse)
		So(tx, ShouldBeNil)
	})
}

func TestWithTxNilHandle(t *testing.T) {
	Convey("WithTx with a nil handle still reports false", t, func() {
		ctx := WithTx(context.Background(), nil)

		tx, ok := TxFromContext(ctx)
		So(ok, ShouldBeFalse)
		So(tx, ShouldBeNil)
	})
}

func TestWithTxScopedToDerivedContext(t *testing.T) {
	Convey("WithTx only affects the derived ctx", t, func() {
		parent := context.Background()
		ctx := WithTx(parent, 42)

		_, ok := TxFromContext(parent)
		So(ok, ShouldBeFalse)

		tx, ok := TxFromContext(ctx)
		So(ok, ShouldBeTrue)
		So(tx, ShouldEqual, 42)
	})
}

func TestWithTxOverridesPreviousHandle(t *testing.T) {
	Convey("nested WithTx returns the innermost handle", t, func() {
		ctx := WithTx(context.Background(), "outer")
		ctx = WithTx(ctx, "inner")

		tx, ok := TxFromContext(ctx)
		So(ok, ShouldBeTrue)
		So(tx, ShouldEqual, "inner")
	})
}

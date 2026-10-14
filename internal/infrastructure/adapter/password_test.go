package adapter

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBcryptHasher(t *testing.T) {
	Convey("BcryptHasher hashes and verifies", t, func() {
		hasher := NewBcryptHasher()
		ctx := t.Context()

		hash, err := hasher.Hash(ctx, "correct horse battery")
		So(err, ShouldBeNil)
		So(hash, ShouldNotEqual, "correct horse battery")
		So(hash, ShouldStartWith, "$2")

		So(hasher.Verify(ctx, hash, "correct horse battery"), ShouldBeNil)
		So(hasher.Verify(ctx, hash, "wrong password"), ShouldNotBeNil)
	})

	Convey("BcryptHasher salts every hash", t, func() {
		hasher := NewBcryptHasher()
		ctx := t.Context()

		first, err := hasher.Hash(ctx, "same-password")
		So(err, ShouldBeNil)
		second, err := hasher.Hash(ctx, "same-password")
		So(err, ShouldBeNil)

		So(first, ShouldNotEqual, second)
	})
}

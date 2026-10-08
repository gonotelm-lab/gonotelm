package redis

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newTestStylePreview(identifier string) *schema.StylePreview {
	return &schema.StylePreview{
		StoreKey:         "stkv1-key",
		Identifier:       identifier,
		Md5sum:           []byte("0123456789abcdef"),
		OriginalFilename: "default.webp",
		CreateTime:       1700000000000,
		UpdateTime:       1700000000001,
	}
}

func TestStylePreviewCacheImplSetGetDelete(t *testing.T) {
	Convey("StylePreviewCache Set/Get/Delete", t, func() {
		ctx := t.Context()
		identifier := "test." + uuid.NewV7().String()

		So(testStylePreviewCache.Set(ctx, identifier, newTestStylePreview(identifier)), ShouldBeNil)

		got, err := testStylePreviewCache.Get(ctx, identifier)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Identifier, ShouldEqual, identifier)
		So(got.StoreKey, ShouldEqual, "stkv1-key")
		So(got.Md5sum, ShouldResemble, []byte("0123456789abcdef"))
		So(got.OriginalFilename, ShouldEqual, "default.webp")
		So(got.CreateTime, ShouldEqual, 1700000000000)
		So(got.UpdateTime, ShouldEqual, 1700000000001)

		So(testStylePreviewCache.Delete(ctx, identifier), ShouldBeNil)

		got, err = testStylePreviewCache.Get(ctx, identifier)
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}

func TestStylePreviewCacheImplGetMissing(t *testing.T) {
	Convey("StylePreviewCache get missing identifier", t, func() {
		got, err := testStylePreviewCache.Get(t.Context(), "test."+uuid.NewV7().String())
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}

func TestStylePreviewCacheImplSetMultiGetMulti(t *testing.T) {
	Convey("StylePreviewCache SetMulti/GetMulti", t, func() {
		ctx := t.Context()
		hitA := "test." + uuid.NewV7().String()
		hitB := "test." + uuid.NewV7().String()
		miss := "test." + uuid.NewV7().String()

		So(testStylePreviewCache.SetMulti(ctx, []*schema.StylePreview{
			newTestStylePreview(hitA),
			newTestStylePreview(hitB),
		}), ShouldBeNil)

		got, err := testStylePreviewCache.GetMulti(ctx, []string{hitA, miss, hitB})
		So(err, ShouldBeNil)
		So(got, ShouldHaveLength, 2)
		So(got[hitA].Identifier, ShouldEqual, hitA)
		So(got[hitB].Identifier, ShouldEqual, hitB)
		_, ok := got[miss]
		So(ok, ShouldBeFalse)

		So(testStylePreviewCache.Delete(ctx, hitA), ShouldBeNil)
		So(testStylePreviewCache.Delete(ctx, hitB), ShouldBeNil)
	})
}

func TestStylePreviewCacheImplSetMultiEmpty(t *testing.T) {
	Convey("StylePreviewCache SetMulti with no previews is a no-op", t, func() {
		So(testStylePreviewCache.SetMulti(t.Context(), nil), ShouldBeNil)
	})
}

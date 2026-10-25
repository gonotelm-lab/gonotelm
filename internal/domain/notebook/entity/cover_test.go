package entity

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeCoverKeyFactory struct {
	bucket string
}

func (f *fakeCoverKeyFactory) New(path string, isPublic bool) (valobj.StoreKey, error) {
	return valobj.NewStoreKey(f.bucket, path, isPublic)
}

func coverKey(factory *fakeCoverKeyFactory, path string, isPublic bool) valobj.StoreKey {
	key, err := factory.New(path, isPublic)
	So(err, ShouldBeNil)
	return key
}

// webpVLHead 构造一个仅含 VP8L 头的 webp 文件（DecodeConfig 只需要头部尺寸位流）。
func webpVLHead(width, height int) []byte {
	payload := make([]byte, 1+4+2) // 0x2f 签名 + 尺寸 bit 流 + 尾部填充
	payload[0] = 0x2f
	binary.LittleEndian.PutUint32(payload[1:], uint32(width-1)|(uint32(height-1)<<14))

	var buf bytes.Buffer
	total := uint32(len("WEBP") + 8 + len(payload)) // riffSize = 去掉 RIFF 头 8B
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, total)
	buf.WriteString("WEBP")
	buf.WriteString("VP8L")
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)

	return buf.Bytes()
}

func pngBytes(width, height int) ([]byte, []byte) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var full bytes.Buffer
	So(png.Encode(&full, img), ShouldBeNil)

	head := full.Bytes()
	if int64(len(head)) > int64(CoverHeadProbeBytes) {
		head = head[:CoverHeadProbeBytes]
	}

	return full.Bytes(), head
}

func jpegHead(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	So(jpeg.Encode(&buf, img, nil), ShouldBeNil)
	return buf.Bytes()
}

func TestCoverContentTypeFromMagic(t *testing.T) {
	Convey("CoverContentTypeFromMagic detects whitelisted image types", t, func() {
		Convey("hits image/png", func() {
			_, head := pngBytes(4, 4)
			mime, ok := CoverContentTypeFromMagic(head)
			So(ok, ShouldBeTrue)
			So(mime, ShouldEqual, "image/png")
		})

		Convey("hits image/jpeg", func() {
			mime, ok := CoverContentTypeFromMagic(jpegHead(4, 4))
			So(ok, ShouldBeTrue)
			So(mime, ShouldEqual, "image/jpeg")
		})

		Convey("hits image/webp", func() {
			mime, ok := CoverContentTypeFromMagic(webpVLHead(4, 4))
			So(ok, ShouldBeTrue)
			So(mime, ShouldEqual, "image/webp")
		})

		Convey("misses garbage bytes", func() {
			mime, ok := CoverContentTypeFromMagic([]byte("this is not an image at all"))
			So(ok, ShouldBeFalse)
			So(mime, ShouldEqual, "")
		})
	})
}

func TestValidateCover(t *testing.T) {
	Convey("ValidateCover accepts a valid small PNG and rejects bad inputs", t, func() {
		full, head := pngBytes(1600, 801)

		Convey("accepts valid 1600x801 png", func() {
			mime, err := ValidateCover(int64(len(full)), head)
			So(err, ShouldBeNil)
			So(mime, ShouldEqual, "image/png")
		})

		Convey("rejects object size over limit", func() {
			_, err := ValidateCover(MaxCoverBytes+1, head)
			So(errors.Is(err, notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("rejects garbage head", func() {
			_, err := ValidateCover(int64(len(full)), []byte("garbage head bytes, not an image"))
			So(errors.Is(err, notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("rejects oversized dimensions", func() {
			full4096, head4096 := pngBytes(4096, 4096)
			_, err := ValidateCover(int64(len(full4096)), head4096)
			So(errors.Is(err, notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("accepts webp head", func() {
			mime, err := ValidateCover(int64(CoverHeadProbeBytes), webpVLHead(1600, 801))
			So(err, ShouldBeNil)
			So(mime, ShouldEqual, "image/webp")
		})
	})
}

func TestCheckCoverKey(t *testing.T) {
	Convey("CheckCoverKey validates public key with cover prefix", t, func() {
		factory := &fakeCoverKeyFactory{bucket: "nb-public"}

		Convey("rejects zero key", func() {
			So(errors.Is(CheckCoverKey(valobj.StoreKey{}), notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("rejects non-public key", func() {
			key := coverKey(factory, CoverObjectPathPrefix+"tok", false)
			So(errors.Is(CheckCoverKey(key), notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("rejects wrong prefix", func() {
			key := coverKey(factory, "notebooks/other/tok", true)
			So(errors.Is(CheckCoverKey(key), notebookerrors.ErrInvalidCover), ShouldBeTrue)
		})

		Convey("accepts valid cover key", func() {
			key := coverKey(factory, CoverObjectPathPrefix+"tok", true)
			So(CheckCoverKey(key), ShouldBeNil)
		})
	})
}

func TestNotebookCoverFlow(t *testing.T) {
	Convey("Notebook cover flow", t, func() {
		factory := &fakeCoverKeyFactory{bucket: "nb-public"}
		ownerId := valobj.NewUid()
		n, err := NewNotebook("nb", ownerId)
		So(err, ShouldBeNil)

		deterministicUpdateTime := func() int64 {
			return valobj.NewTimeFrom(1000).Value()
		}

		Convey("BeginCoverFlow issues a new pending key", func() {
			n.UpdateTime = valobj.NewTimeFrom(deterministicUpdateTime())

			key, isNew, err := n.BeginCoverFlow(factory)
			So(err, ShouldBeNil)
			So(isNew, ShouldBeTrue)
			So(key.IsPublic, ShouldBeTrue)
			So(key.Key, ShouldStartWith, CoverObjectPathPrefix)
			So(n.PendingCover, ShouldResemble, key)
			So(n.UpdateTime.Value(), ShouldEqual, deterministicUpdateTime())
			So(n.Cover.Valid(), ShouldBeFalse)
		})

		Convey("BeginCoverFlow reuses existing pending key", func() {
			existing, _, err := n.BeginCoverFlow(factory)
			So(err, ShouldBeNil)
			n.UpdateTime = valobj.NewTimeFrom(deterministicUpdateTime())

			key, isNew, err := n.BeginCoverFlow(factory)
			So(err, ShouldBeNil)
			So(isNew, ShouldBeFalse)
			So(key, ShouldResemble, existing)
			So(n.PendingCover, ShouldResemble, existing)
			So(n.UpdateTime.Value(), ShouldEqual, deterministicUpdateTime())
			So(n.Cover.Valid(), ShouldBeFalse)
		})

		Convey("SetCover rejects invalid key", func() {
			So(errors.Is(n.SetCover(valobj.StoreKey{}), notebookerrors.ErrInvalidCover), ShouldBeTrue)

			private := coverKey(factory, CoverObjectPathPrefix+"tok", false)
			So(errors.Is(n.SetCover(private), notebookerrors.ErrInvalidCover), ShouldBeTrue)

			So(n.Cover.Valid(), ShouldBeFalse)
		})

		Convey("SetCover confirms pending key and clears it", func() {
			pending, _, err := n.BeginCoverFlow(factory)
			So(err, ShouldBeNil)
			n.UpdateTime = valobj.NewTimeFrom(deterministicUpdateTime())

			So(n.SetCover(pending), ShouldBeNil)
			So(n.Cover, ShouldResemble, pending)
			So(n.PendingCover.Valid(), ShouldBeFalse)
			So(n.UpdateTime.Value(), ShouldBeGreaterThan, deterministicUpdateTime())
		})

		Convey("ClearCover clears both cover and pending", func() {
			cover := coverKey(factory, CoverObjectPathPrefix+"active", true)
			n.Cover = cover
			n.PendingCover = coverKey(factory, CoverObjectPathPrefix+"flying", true)
			n.UpdateTime = valobj.NewTimeFrom(deterministicUpdateTime())

			n.ClearCover()
			So(n.Cover.Valid(), ShouldBeFalse)
			So(n.PendingCover.Valid(), ShouldBeFalse)
			So(n.UpdateTime.Value(), ShouldBeGreaterThan, deterministicUpdateTime())
		})

		Convey("ClearPendingCover only clears pending cover", func() {
			cover := coverKey(factory, CoverObjectPathPrefix+"active", true)
			n.Cover = cover
			n.PendingCover = coverKey(factory, CoverObjectPathPrefix+"flying", true)
			n.UpdateTime = valobj.NewTimeFrom(deterministicUpdateTime())

			n.ClearPendingCover()
			So(n.Cover, ShouldResemble, cover)
			So(n.PendingCover.Valid(), ShouldBeFalse)
			So(n.UpdateTime.Value(), ShouldEqual, deterministicUpdateTime())
		})
	})
}

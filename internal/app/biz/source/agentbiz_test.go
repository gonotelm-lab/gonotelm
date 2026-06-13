package source

import (
	"context"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBizForAgent_ReadSource_CacheMissUsesPatchedFetch(t *testing.T) {
	Convey("cache miss时调用mock fetch并缓存多行结果", t, func() {
		biz, err := NewAgentBiz(context.Background(), nil, AgentBizConfig{
			SourceCacheEviction: time.Minute,
			SourceCacheMaxMB:    1,
		})
		So(err, ShouldBeNil)
		So(biz, ShouldNotBeNil)

		sourceID := uuid.NewV4()
		called := 0

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(biz, "fetchSourceContent",
			func(_ *AgentBiz, _ context.Context, gotSourceID uuid.UUID) ([]byte, string, error) {
				called++
				So(gotSourceID.EqualsTo(sourceID), ShouldBeTrue)
				return []byte("core-line-1\ncore-line-2\ncore-line-3"), "abstract", nil
			})

		got, err := biz.ReadSource(context.Background(), &AgentReadSourceQuery{
			SourceId: sourceID,
		})
		So(err, ShouldBeNil)
		So(called, ShouldEqual, 1)
		So(got, ShouldNotBeNil)
		So(got.TotalLines, ShouldEqual, 3)
		So(len(got.Lines), ShouldEqual, 3)
		So(got.Lines[0].LineNo, ShouldEqual, 1)
		So(string(got.Lines[0].Line), ShouldEqual, "core-line-1")
		So(got.Lines[1].LineNo, ShouldEqual, 2)
		So(string(got.Lines[1].Line), ShouldEqual, "core-line-2")
		So(got.Lines[2].LineNo, ShouldEqual, 3)
		So(string(got.Lines[2].Line), ShouldEqual, "core-line-3")

		// 第二次读取应走缓存，不再访问 fetchSourceContent。
		got, err = biz.ReadSource(context.Background(), &AgentReadSourceQuery{
			SourceId: sourceID,
			Offset:   2,
			Limit:    1,
		})
		So(err, ShouldBeNil)
		So(called, ShouldEqual, 1)
		So(got, ShouldNotBeNil)
		So(got.TotalLines, ShouldEqual, 3)
		So(len(got.Lines), ShouldEqual, 1)
		So(got.Lines[0].LineNo, ShouldEqual, 2)
		So(string(got.Lines[0].Line), ShouldEqual, "core-line-2")
	})
}

func TestBizForAgent_ReadSource_CacheHitSkipsFetchAndAppliesRange(t *testing.T) {
	Convey("cache hit时跳过fetch并按范围返回", t, func() {
		biz, err := NewAgentBiz(context.Background(), nil, AgentBizConfig{
			SourceCacheEviction: time.Minute,
			SourceCacheMaxMB:    4,
		})
		So(err, ShouldBeNil)
		So(biz, ShouldNotBeNil)

		sourceID := uuid.NewV4()
		content := []byte("line-1\nline-2\nline-3")
		payload, err := biz.encodeCachedSource(content, biz.buildLineRanges(content), "abstract")
		So(err, ShouldBeNil)
		err = biz.sourceCache.Set(sourceID.String(), payload)
		So(err, ShouldBeNil)

		called := 0
		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(biz, "fetchSourceContent",
			func(_ *AgentBiz, _ context.Context, _ uuid.UUID) ([]byte, string, error) {
				called++
				return []byte("mock-line-1\nmock-line-2"), "abstract", nil
			})

		got, err := biz.ReadSource(context.Background(), &AgentReadSourceQuery{
			SourceId: sourceID,
			Offset:   2,
			Limit:    1,
		})
		So(err, ShouldBeNil)
		So(called, ShouldEqual, 0)
		So(got, ShouldNotBeNil)
		So(got.TotalLines, ShouldEqual, 3)
		So(len(got.Lines), ShouldEqual, 1)
		So(got.Lines[0].LineNo, ShouldEqual, 2)
		So(string(got.Lines[0].Line), ShouldEqual, "line-2")
	})
}

func TestBizForAgent_StatSource_CacheMissUsesPatchedFetch(t *testing.T) {
	Convey("StatSource在cache miss时调用mock fetch并在后续命中缓存", t, func() {
		biz, err := NewAgentBiz(context.Background(), nil, AgentBizConfig{
			SourceCacheEviction: time.Minute,
			SourceCacheMaxMB:    1,
		})
		So(err, ShouldBeNil)
		So(biz, ShouldNotBeNil)

		sourceID := uuid.NewV4()
		called := 0
		expectedContent := []byte("你好a\n世界b")

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(biz, "fetchSourceContent",
			func(_ *AgentBiz, _ context.Context, gotSourceID uuid.UUID) ([]byte, string, error) {
				called++
				So(gotSourceID.EqualsTo(sourceID), ShouldBeTrue)
				return expectedContent, "abstract", nil
			})

		got, err := biz.StatSource(context.Background(), sourceID)
		So(err, ShouldBeNil)
		So(called, ShouldEqual, 1)
		So(got, ShouldNotBeNil)
		So(got.Bytes, ShouldEqual, len(expectedContent))
		So(got.Runes, ShouldEqual, 7)
		So(got.Lines, ShouldEqual, 2)

		// 第二次读取应命中缓存，不再访问 fetchSourceContent。
		got, err = biz.StatSource(context.Background(), sourceID)
		So(err, ShouldBeNil)
		So(called, ShouldEqual, 1)
		So(got, ShouldNotBeNil)
		So(got.Bytes, ShouldEqual, len(expectedContent))
		So(got.Runes, ShouldEqual, 7)
		So(got.Lines, ShouldEqual, 2)
	})
}

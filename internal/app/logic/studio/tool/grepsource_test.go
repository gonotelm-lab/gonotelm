package tool

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGrepSourceTool_InvokableRun_GetSourceContentFailed(t *testing.T) {
	Convey("InvokableRun 在 GetSourceContent 失败时返回包装错误", t, func() {
		sourceID := uuid.NewV4()
		biz := &bizsource.BizForAgent{}

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		mockErr := errors.New("mock get source content failed")
		patches.ApplyMethodReturn(biz, "GetSourceContent", []byte(nil), mockErr)

		tool := NewGrepSourceTool(biz)
		got, err := tool.InvokableRun(context.Background(), fmt.Sprintf(
			`{"source_id":"%s","pattern":"hello"}`,
			sourceID.String(),
		))

		So(got, ShouldEqual, "")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "get source content failed")
		So(err.Error(), ShouldContainSubstring, mockErr.Error())
	})
}

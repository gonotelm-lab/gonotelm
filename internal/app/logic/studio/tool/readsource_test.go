package tool

import (
	"context"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/bytedance/sonic"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestReadSourceTool_Info(t *testing.T) {
	s, _ := readSourceToolParams.ToJSONSchema()
	json, err := sonic.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal jsonschema failed: %v", err)
	}
	t.Log(string(json))
	t.Log("-------------------------")
	s, _ = grepSourceToolParams.ToJSONSchema()
	json, _ = sonic.MarshalIndent(s, "", "  ")
	t.Log(string(json))
}

func TestReadSourceTool_InvokableRun(t *testing.T) {
	Convey("InvokableRun 使用 start_line 和 line_count 参数格式化输出", t, func() {
		sourceID := uuid.NewV4()
		biz := &bizsource.BizForAgent{}

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethodReturn(biz, "ReadSource",
			&bizsource.ReadSourceResult{
				Lines: []bizsource.ReadSourceResultLine{
					{LineNo: 2, Line: []byte("hello")},
					{LineNo: 3, Line: []byte("world")},
				},
			},
			nil,
		)

		tool := NewReadSourceTool(biz, nil)
		got, err := tool.InvokableRun(context.Background(), fmt.Sprintf(
			`{"source_id":"%s","start_line":2,"line_count":2}`,
			sourceID.String(),
		))

		So(err, ShouldBeNil)
		So(got, ShouldEqual, "2|hello\n3|world\n")
	})
}

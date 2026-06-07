package rg

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/components/tool/utils"
)

func TestGrepSchema(t *testing.T) {
	s, _ := utils.GoStruct2ParamsOneOf[Params]()
	sch, err := s.ToJSONSchema()
	t.Log(err)
	out, err := json.MarshalIndent(sch, "", " ")
	t.Log(err)
	t.Log(string(out))
}

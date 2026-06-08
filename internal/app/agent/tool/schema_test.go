package tool

import (
	"testing"

	"github.com/bytedance/sonic"
)

func TestReadSourceToolSchema(t *testing.T) {
	s, _ := readSourceToolParams.ToJSONSchema()
	json, err := sonic.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal jsonschema failed: %v", err)
	}
	t.Log(string(json))
}

func TestGrepSourceToolSchema(t *testing.T) {
	s, _ := grepSourceToolParams.ToJSONSchema()
	json, err := sonic.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal jsonschema failed: %v", err)
	}
	t.Log(string(json))
}

func TestQuerySourceToolSchema(t *testing.T) {
	s, _ := querySourceToolParams.ToJSONSchema()
	json, err := sonic.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal jsonschema failed: %v", err)
	}
	t.Log(string(json))
}

func TestStatSourceToolSchema(t *testing.T) {
	s, _ := statSourceToolParams.ToJSONSchema()
	json, err := sonic.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("marshal jsonschema failed: %v", err)
	}
	t.Log(string(json))
}

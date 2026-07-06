package schema

import (
	"encoding/json"
	"math"
	"testing"
)

func TestSourceDocGetInt64Meta(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  *int64
		ok    bool
	}{
		{name: "int64", value: int64(7), want: int64Ptr(7), ok: true},
		{name: "float64 integral", value: float64(9), want: int64Ptr(9), ok: true},
		{name: "json number int", value: json.Number("11"), want: int64Ptr(11), ok: true},
		{name: "json number float integral", value: json.Number("13.0"), want: int64Ptr(13), ok: true},
		{name: "float64 fraction", value: float64(2.5), want: int64Ptr(2), ok: true},
		{name: "string number", value: "15", want: int64Ptr(15), ok: true},
		{name: "nan", value: math.NaN(), want: nil, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &SourceDoc{Meta: map[string]any{"k": tt.value}}
			got, ok := doc.GetInt64Meta("k")
			if ok != tt.ok {
				t.Fatalf("unexpected ok, want=%v got=%v", tt.ok, ok)
			}
			if tt.want != nil && got != *tt.want {
				t.Fatalf("unexpected value, want=%d got=%d", *tt.want, got)
			}
		})
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestSourceDocGetFloat64Meta(t *testing.T) {
	doc := &SourceDoc{
		Meta: map[string]any{
			"int":  int64(8),
			"json": json.Number("6.5"),
			"bad":  "oops",
		},
	}

	gotInt, ok := doc.GetFloat64Meta("int")
	if !ok || gotInt != 8 {
		t.Fatalf("expected int meta cast to float64, got=%v ok=%v", gotInt, ok)
	}

	gotJSON, ok := doc.GetFloat64Meta("json")
	if !ok || gotJSON != 6.5 {
		t.Fatalf("expected json number meta cast to float64, got=%v ok=%v", gotJSON, ok)
	}

	if _, ok := doc.GetFloat64Meta("bad"); ok {
		t.Fatalf("expected bad meta type to be rejected")
	}
}

func TestSourceDocMetaGettersCast(t *testing.T) {
	doc := &SourceDoc{
		Meta: map[string]any{
			"s":    123,
			"b":    "true",
			"badb": "not-bool",
		},
	}

	if value, ok := doc.GetStringMeta("s"); !ok || value != "123" {
		t.Fatalf("expected string meta cast by spf13/cast, got=%q ok=%v", value, ok)
	}
	if value, ok := doc.GetBoolMeta("b"); !ok || !value {
		t.Fatalf("expected bool meta cast by spf13/cast, got=%v ok=%v", value, ok)
	}
	if value, ok := doc.GetMetaBool("b"); !ok || !value {
		t.Fatalf("expected meta bool cast by spf13/cast, got=%v ok=%v", value, ok)
	}
	if _, ok := doc.GetBoolMeta("badb"); ok {
		t.Fatalf("expected invalid bool string to return false")
	}
}

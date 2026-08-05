package s3conv

import (
	"testing"
)

func TestSpanName(t *testing.T) {
	if got := SpanName("GetObject"); got != "S3.GetObject" {
		t.Fatalf("SpanName: got %q want %q", got, "S3.GetObject")
	}
}

func TestMethodName(t *testing.T) {
	if got := MethodName("GetObject"); got != "S3/GetObject" {
		t.Fatalf("MethodName: got %q want %q", got, "S3/GetObject")
	}
}

func TestAttributes(t *testing.T) {
	attrs := Attributes("GetObject", "bucket-1", "us-east-1", "key-1")
	want := map[string]string{
		"aws.s3.bucket":   "bucket-1",
		"rpc.system.name": "aws-api",
		"rpc.method":      "S3/GetObject",
		"aws.region":      "us-east-1",
		"aws.s3.key":      "key-1",
	}
	got := make(map[string]string)
	for _, a := range attrs {
		got[string(a.Key)] = a.Value.AsString()
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("attr %s: got %q want %q (attrs=%v)", k, got[k], v, attrs)
		}
	}
}

func TestAttributesOmitsEmptyRegionAndKey(t *testing.T) {
	attrs := Attributes("StatObject", "bucket-1", "", "")
	for _, a := range attrs {
		if string(a.Key) == "aws.region" || string(a.Key) == "aws.s3.key" {
			t.Fatalf("attr %s should be omitted: %v", a.Key, attrs)
		}
	}
}

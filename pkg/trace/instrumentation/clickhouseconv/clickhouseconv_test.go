package clickhouseconv

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

func attrMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func TestAppendSpanName(t *testing.T) {
	if got := AppendSpanName("INSERT INTO events(id)"); got != "INSERT events" {
		t.Errorf("AppendSpanName: got %q want %q", got, "INSERT events")
	}
}

func TestAppendAttributes(t *testing.T) {
	attrs := attrMap(AppendAttributes("INSERT INTO events(id)"))

	if got := attrs[semconv.DBSystemNameKey]; got.AsString() != "clickhouse" {
		t.Errorf("db.system.name: got %q want %q", got.AsString(), "clickhouse")
	}
	wantStr := map[attribute.Key]string{
		"db.operation.name": "INSERT",
		"db.collection.name": "events",
		"db.query.summary":   "INSERT events",
		"db.query.text":      "INSERT INTO events(id)",
	}
	for k, wantV := range wantStr {
		if got := attrs[k]; got.AsString() != wantV {
			t.Errorf("attribute %s: got %q want %q", k, got.AsString(), wantV)
		}
	}
}

func TestAppendAttributesQualifiedTable(t *testing.T) {
	attrs := attrMap(AppendAttributes("INSERT INTO default.events(id)"))

	if got := attrs[semconv.DBNamespaceKey].AsString(); got != "default" {
		t.Errorf("db.namespace: got %q want %q", got, "default")
	}
	if got := attrs[semconv.DBCollectionNameKey].AsString(); got != "events" {
		t.Errorf("db.collection.name: got %q want %q", got, "events")
	}
}

func TestParseInsertQueryQuotedTable(t *testing.T) {
	meta := ParseInsertQuery(`INSERT INTO "event log"(id)`)
	if meta.Collection != "event log" {
		t.Fatalf("collection: got %q want %q", meta.Collection, "event log")
	}
	if meta.Summary != `INSERT event log` {
		t.Fatalf("summary: got %q want %q", meta.Summary, "INSERT event log")
	}
}

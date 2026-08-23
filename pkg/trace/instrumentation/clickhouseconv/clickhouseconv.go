package clickhouseconv

import (
	"strings"
	"unicode"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// QueryMeta holds parsed INSERT query metadata for database semconv attributes.
type QueryMeta struct {
	Operation  string // db.operation.name
	Namespace  string // db.namespace
	Collection string // db.collection.name
	Summary    string // db.query.summary
}

// AppendSpanName returns the database span name, preferring db.query.summary.
func AppendSpanName(query string) string {
	meta := ParseInsertQuery(query)
	if meta.Summary != "" {
		return meta.Summary
	}
	if meta.Operation != "" && meta.Collection != "" {
		return meta.Operation + " " + meta.Collection
	}
	if meta.Operation != "" {
		return meta.Operation
	}
	return semconv.DBSystemNameClickHouse.Value.AsString()
}

// AppendAttributes returns database client span attributes for batch append.
func AppendAttributes(query string) []attribute.KeyValue {
	meta := ParseInsertQuery(query)

	attrs := []attribute.KeyValue{semconv.DBSystemNameClickHouse}
	if meta.Operation != "" {
		attrs = append(attrs, semconv.DBOperationName(meta.Operation))
	}
	if meta.Namespace != "" {
		attrs = append(attrs, semconv.DBNamespace(meta.Namespace))
	}
	if meta.Collection != "" {
		attrs = append(attrs, semconv.DBCollectionName(meta.Collection))
	}
	if meta.Summary != "" {
		attrs = append(attrs, semconv.DBQuerySummary(meta.Summary))
	}
	if query != "" {
		attrs = append(attrs, semconv.DBQueryText(query))
	}
	return attrs
}

// AppendSpanStartOpts returns span start options for batch append.
func AppendSpanStartOpts(query string) []oteltrace.SpanStartOption {
	return []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(AppendAttributes(query)...),
	}
}

// ParseInsertQuery extracts database semconv fields from a ClickHouse INSERT query.
func ParseInsertQuery(query string) QueryMeta {
	q := strings.TrimSpace(query)
	if q == "" {
		return QueryMeta{}
	}

	const prefix = "INSERT INTO "
	if len(q) < len(prefix) || !strings.EqualFold(q[:len(prefix)], prefix) {
		return QueryMeta{}
	}

	namespace, collection := parseInsertTarget(strings.TrimSpace(q[len(prefix):]))
	meta := QueryMeta{
		Operation:  "INSERT",
		Namespace:  namespace,
		Collection: collection,
	}
	if collection != "" {
		meta.Summary = "INSERT " + collection
	} else {
		meta.Summary = "INSERT"
	}
	return meta
}

func parseInsertTarget(s string) (namespace, collection string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}

	if c := s[0]; c == '`' || c == '"' {
		end := strings.IndexByte(s[1:], c)
		if end < 0 {
			return "", ""
		}
		return "", s[1 : 1+end]
	}

	end := len(s)
	for i, r := range s {
		if r == '(' || unicode.IsSpace(r) {
			end = i
			break
		}
	}
	target := s[:end]
	if dot := strings.LastIndex(target, "."); dot >= 0 {
		return target[:dot], target[dot+1:]
	}
	return "", target
}

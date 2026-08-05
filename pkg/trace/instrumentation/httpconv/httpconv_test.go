package httpconv

import (
	"crypto/tls"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func attrMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func TestServerAttributes(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://example.com:8443/api/users?id=1", nil)
	req.RemoteAddr = "203.0.113.7:52341"
	req.Header.Set("User-Agent", "test-agent/1.0")

	attrs := attrMap(ServerAttributes("", "/api/users", req))

	want := map[attribute.Key]string{
		"http.request.method":      "POST",
		"url.path":                 "/api/users",
		"url.scheme":               "https",
		"http.route":               "/api/users",
		"server.address":           "example.com",
		"network.peer.address":     "203.0.113.7",
		"client.address":           "203.0.113.7",
		"network.protocol.version": "1.1",
		"user_agent.original":      "test-agent/1.0",
	}
	for k, wantV := range want {
		if got := attrs[k]; got.AsString() != wantV {
			t.Errorf("attribute %s: got %q want %q", k, got.AsString(), wantV)
		}
	}
	if got := attrs["server.port"].AsInt64(); got != 8443 {
		t.Errorf("server.port: got %d want 8443", got)
	}
	if got := attrs["network.peer.port"].AsInt64(); got != 52341 {
		t.Errorf("network.peer.port: got %d want 52341", got)
	}
	// http is the default protocol name, so network.protocol.name is omitted
	if got := attrs["network.protocol.name"]; got.Type() != attribute.EMPTY {
		t.Errorf("network.protocol.name should be omitted for http, got %v", got)
	}
}

func TestServerAttributesDefaultPortsOmitted(t *testing.T) {
	// default port 80 is omitted for http
	req80, _ := http.NewRequest("GET", "http://example.com:80/api", nil)
	attrs80 := attrMap(ServerAttributes("", "", req80))
	if got := attrs80["server.port"]; got.Type() != attribute.EMPTY {
		t.Errorf("server.port 80 should be omitted for http, got %v", got)
	}

	// default port 443 is omitted for https
	req443, _ := http.NewRequest("GET", "https://example.com:443/api", nil)
	attrs443 := attrMap(ServerAttributes("", "", req443))
	if got := attrs443["server.port"]; got.Type() != attribute.EMPTY {
		t.Errorf("server.port 443 should be omitted for https, got %v", got)
	}
}

func TestServerAttributesSchemeFallback(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	req.URL.Scheme = ""
	req.Host = "localhost"

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["url.scheme"].AsString(); got != "http" {
		t.Errorf("url.scheme fallback: got %q want %q", got, "http")
	}
	// no server.port when host has no port
	if got := attrs["server.port"]; got.Type() != attribute.EMPTY {
		t.Errorf("server.port without port should not be set, got %v", got)
	}
}

func TestServerAttributesServerNameFallback(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	req.Host = ""

	attrs := attrMap(ServerAttributes("api-gateway", "", req))
	if got := attrs["server.address"].AsString(); got != "api-gateway" {
		t.Errorf("server.address should fall back to server name, got %q want %q", got, "api-gateway")
	}
}

func TestServerAttributesTLS(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	req.URL.Scheme = ""
	req.Host = "localhost"
	req.TLS = &tls.ConnectionState{}

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["url.scheme"].AsString(); got != "https" {
		t.Errorf("url.scheme with TLS: got %q want %q", got, "https")
	}
}

func TestServerAttributesUnknownMethod(t *testing.T) {
	req, _ := http.NewRequest("PURGE", "http://localhost/api", nil)

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["http.request.method"].AsString(); got != "_OTHER" {
		t.Errorf("unknown method: got %q want %q", got, "_OTHER")
	}
	if got := attrs["http.request.method_original"].AsString(); got != "PURGE" {
		t.Errorf("method_original: got %q want %q", got, "PURGE")
	}
}

func TestServerAttributesCaseInsensitiveMethod(t *testing.T) {
	req, _ := http.NewRequest("get", "http://localhost/api", nil)

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["http.request.method"].AsString(); got != "GET" {
		t.Errorf("lowercase method should be canonicalized: got %q want %q", got, "GET")
	}
	if got := attrs["http.request.method_original"].AsString(); got != "get" {
		t.Errorf("method_original: got %q want %q", got, "get")
	}
}

func TestServerAttributesXForwardedFor(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.1")

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["client.address"].AsString(); got != "198.51.100.1" {
		t.Errorf("client.address: got %q want %q", got, "198.51.100.1")
	}
	if got := attrs["network.peer.address"].AsString(); got != "10.0.0.1" {
		t.Errorf("network.peer.address: got %q want %q", got, "10.0.0.1")
	}
}

func TestServerAttributesIPv6Host(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)
	req.Host = "[::1]:8080"

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["server.address"].AsString(); got != "::1" {
		t.Errorf("server.address: got %q want %q", got, "::1")
	}
	if got := attrs["server.port"].AsInt64(); got != 8080 {
		t.Errorf("server.port: got %d want 8080", got)
	}

	reqNoPort, _ := http.NewRequest("GET", "http://localhost/api", nil)
	reqNoPort.Host = "[::1]"
	attrsNoPort := attrMap(ServerAttributes("", "", reqNoPort))
	if got := attrsNoPort["server.address"].AsString(); got != "::1" {
		t.Errorf("server.address no port: got %q want %q", got, "::1")
	}
	if got := attrsNoPort["server.port"]; got.Type() != attribute.EMPTY {
		t.Errorf("server.port should not be set, got %v", got)
	}
}

func TestServerAttributesNoUserAgent(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/api", nil)

	attrs := attrMap(ServerAttributes("", "", req))
	if got := attrs["user_agent.original"]; got.Type() != attribute.EMPTY {
		t.Errorf("user_agent.original should not be set when absent, got %v", got)
	}
}

func TestResponseAttributes(t *testing.T) {
	attrs := attrMap(ResponseAttributes(200))
	if got := attrs["http.response.status_code"].AsInt64(); got != 200 {
		t.Errorf("status code: got %d want 200", got)
	}
	if got := attrs["error.type"]; got.Type() != attribute.EMPTY {
		t.Errorf("error.type must not be set on 2xx, got %v", got)
	}

	attrs500 := attrMap(ResponseAttributes(500))
	if got := attrs500["error.type"].AsString(); got != "500" {
		t.Errorf("error.type on 5xx: got %q want %q", got, "500")
	}
}

func TestSpanStatus(t *testing.T) {
	cases := []struct {
		name string
		code int
		kind oteltrace.SpanKind
		want codes.Code
	}{
		{"invalid low", 99, oteltrace.SpanKindServer, codes.Error},
		{"invalid high", 600, oteltrace.SpanKindServer, codes.Error},
		{"1xx server", 100, oteltrace.SpanKindServer, codes.Unset},
		{"2xx server", 200, oteltrace.SpanKindServer, codes.Unset},
		{"3xx server", 302, oteltrace.SpanKindServer, codes.Unset},
		{"4xx server", 404, oteltrace.SpanKindServer, codes.Unset},
		{"4xx client", 404, oteltrace.SpanKindClient, codes.Error},
		{"5xx server", 503, oteltrace.SpanKindServer, codes.Error},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, desc := SpanStatus(c.code, c.kind)
			if got != c.want {
				t.Errorf("SpanStatus(%d, %v): got %v want %v", c.code, c.kind, got, c.want)
			}
			if c.want == codes.Error && c.code >= 600 {
				if desc == "" {
					t.Errorf("invalid status code should have description, got empty")
				}
			} else if c.want != codes.Error && desc != "" {
				t.Errorf("non-error status should have empty description, got %q", desc)
			}
		})
	}
}

package httpconv

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var methodLookup = map[string]attribute.KeyValue{
	http.MethodConnect: semconv.HTTPRequestMethodConnect,
	http.MethodDelete:  semconv.HTTPRequestMethodDelete,
	http.MethodGet:     semconv.HTTPRequestMethodGet,
	http.MethodHead:    semconv.HTTPRequestMethodHead,
	http.MethodOptions: semconv.HTTPRequestMethodOptions,
	http.MethodPatch:   semconv.HTTPRequestMethodPatch,
	http.MethodPost:    semconv.HTTPRequestMethodPost,
	http.MethodPut:     semconv.HTTPRequestMethodPut,
	http.MethodTrace:   semconv.HTTPRequestMethodTrace,
}

// ServerAttributes returns the stable HTTP server semantic convention attributes (call at span start).
// server is the primary server name used for server.address when req.Host is absent; route is the
// low-cardinality route template (e.g. hertz FullPath()); req is the compatible net/http request.
func ServerAttributes(server, route string, req *http.Request) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 12)

	method, methodOriginal := method(req.Method)
	attrs = append(attrs, method)
	if methodOriginal != (attribute.KeyValue{}) {
		attrs = append(attrs, methodOriginal)
	}

	scheme := ""
	if req.URL != nil && req.URL.Scheme != "" {
		scheme = req.URL.Scheme
	} else if req.TLS != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}
	https := scheme == "https"
	attrs = append(attrs, semconv.URLSchemeKey.String(scheme))

	host := req.Host
	if host == "" {
		host = server
	}
	if host, port := splitHostPort(host); host != "" {
		attrs = append(attrs, semconv.ServerAddressKey.String(host))
		if p := requiredHTTPPort(https, port); p > 0 {
			attrs = append(attrs, semconv.ServerPortKey.Int(p))
		}
	}

	if peer, peerPort := splitHostPort(req.RemoteAddr); peer != "" {
		attrs = append(attrs, semconv.NetworkPeerAddressKey.String(peer))
		if peerPort > 0 {
			attrs = append(attrs, semconv.NetworkPeerPortKey.Int(peerPort))
		}
	}

	if clientIP := serverClientIP(req.Header.Get("X-Forwarded-For")); clientIP != "" {
		attrs = append(attrs, semconv.ClientAddressKey.String(clientIP))
	} else if peer, _ := splitHostPort(req.RemoteAddr); peer != "" {
		attrs = append(attrs, semconv.ClientAddressKey.String(peer))
	}

	if req.URL != nil && req.URL.Path != "" {
		attrs = append(attrs, semconv.URLPathKey.String(req.URL.Path))
	}

	protoName, protoVersion := netProtocol(req.Proto)
	if protoName != "" && protoName != "http" {
		attrs = append(attrs, semconv.NetworkProtocolNameKey.String(protoName))
	}
	if protoVersion != "" {
		attrs = append(attrs, semconv.NetworkProtocolVersionKey.String(protoVersion))
	}

	if route != "" {
		attrs = append(attrs, semconv.HTTPRouteKey.String(route))
	}

	if ua := req.UserAgent(); ua != "" {
		attrs = append(attrs, semconv.UserAgentOriginalKey.String(ua))
	}

	return attrs
}

// ResponseAttributes returns the response-side semantic convention attributes (call at span end).
// For code >= 500 it also sets error.type to the code as a string.
func ResponseAttributes(code int) []attribute.KeyValue {
	attrs := []attribute.KeyValue{semconv.HTTPResponseStatusCodeKey.Int(code)}
	if code >= 500 {
		attrs = append(attrs, semconv.ErrorTypeKey.String(strconv.Itoa(code)))
	}
	return attrs
}

// SpanStatus maps an HTTP status code to a span status.
// Invalid codes (<100 or >=600) and 5xx map to Error; 4xx maps to Error only for CLIENT spans.
func SpanStatus(code int, kind oteltrace.SpanKind) (codes.Code, string) {
	if code < 100 || code >= 600 {
		return codes.Error, "Invalid HTTP status code " + strconv.Itoa(code)
	}
	switch {
	case code >= 500:
		return codes.Error, ""
	case code >= 400 && kind == oteltrace.SpanKindClient:
		return codes.Error, ""
	default:
		return codes.Unset, ""
	}
}

// method returns the http.request.method attribute, plus method_original when the method is non-canonical or unknown.
func method(method string) (attribute.KeyValue, attribute.KeyValue) {
	if method == "" {
		return semconv.HTTPRequestMethodOther, attribute.KeyValue{}
	}
	if attr, ok := methodLookup[method]; ok {
		return attr, attribute.KeyValue{}
	}
	orig := semconv.HTTPRequestMethodOriginalKey.String(method)
	if attr, ok := methodLookup[strings.ToUpper(method)]; ok {
		return attr, orig
	}
	return semconv.HTTPRequestMethodOther, orig
}

// splitHostPort splits "host:port"; port is -1 when missing or unparsable.
// Bracket-enclosed IPv6 hosts without a port (e.g. "[::1]") are handled.
func splitHostPort(hostport string) (host string, port int) {
	port = -1
	if hostport == "" {
		return "", port
	}
	if strings.HasPrefix(hostport, "[") {
		addrEnd := strings.LastIndexByte(hostport, ']')
		if addrEnd < 0 {
			return "", port
		}
		if i := strings.LastIndexByte(hostport[addrEnd:], ':'); i < 0 {
			return hostport[1:addrEnd], port
		}
	} else if i := strings.LastIndexByte(hostport, ':'); i < 0 {
		return hostport, port
	}
	host, pStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", port
	}
	p, err := strconv.ParseUint(pStr, 10, 16)
	if err != nil {
		return host, port
	}
	return host, int(p)
}

// requiredHTTPPort omits the default port (80/443) from server.port.
func requiredHTTPPort(https bool, port int) int {
	if https {
		if port > 0 && port != 443 {
			return port
		}
	} else {
		if port > 0 && port != 80 {
			return port
		}
	}
	return -1
}

// serverClientIP returns the first address in X-Forwarded-For.
func serverClientIP(xForwardedFor string) string {
	if idx := strings.IndexByte(xForwardedFor, ','); idx >= 0 {
		xForwardedFor = xForwardedFor[:idx]
	}
	return xForwardedFor
}

// netProtocol parses "HTTP/1.1" into name="http", version="1.1".
func netProtocol(proto string) (name, version string) {
	name, version, _ = strings.Cut(proto, "/")
	switch name {
	case "HTTP":
		name = "http"
	case "QUIC":
		name = "quic"
	case "SPDY":
		name = "spdy"
	default:
		name = strings.ToLower(name)
	}
	return name, version
}

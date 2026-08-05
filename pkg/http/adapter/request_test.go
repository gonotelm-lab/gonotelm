package adapter

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
)

func TestGetCompatRequestFillsHost(t *testing.T) {
	req := protocol.NewRequest("GET", "/api/users", nil)
	req.Header.SetHost("example.com")

	r, err := GetCompatRequest(req, "")
	if err != nil {
		t.Fatalf("GetCompatRequest() failed: %v", err)
	}
	if r.Host != "example.com" {
		t.Errorf("Host: got %q want %q", r.Host, "example.com")
	}
}

func TestGetCompatRequestKeepsURIHost(t *testing.T) {
	req := protocol.NewRequest("GET", "http://api.example.com/users", nil)

	r, err := GetCompatRequest(req, "")
	if err != nil {
		t.Fatalf("GetCompatRequest() failed: %v", err)
	}
	if r.Host != "api.example.com" {
		t.Errorf("Host: got %q want %q", r.Host, "api.example.com")
	}
}

func TestGetCompatRequestFillsRemoteAddr(t *testing.T) {
	req := protocol.NewRequest("GET", "/api/users", nil)

	r, err := GetCompatRequest(req, "203.0.113.7:52341")
	if err != nil {
		t.Fatalf("GetCompatRequest() failed: %v", err)
	}
	if r.RemoteAddr != "203.0.113.7:52341" {
		t.Errorf("RemoteAddr: got %q want %q", r.RemoteAddr, "203.0.113.7:52341")
	}
}

func TestGetCompatRequestHostHeader(t *testing.T) {
	req := protocol.NewRequest("POST", "/api", nil)
	req.Header.Set("X-Custom", "v1")

	r, err := GetCompatRequest(req, "")
	if err != nil {
		t.Fatalf("GetCompatRequest() failed: %v", err)
	}
	if got := r.Header.Get("X-Custom"); got != "v1" {
		t.Errorf("header: got %q want %q", got, "v1")
	}
	if r.Header.Get("Host") != "" {
		t.Errorf("Host header should not be duplicated in r.Header, got %q", r.Header.Get("Host"))
	}
}

package requestid

import "testing"

func TestGenParseRoundTrip(t *testing.T) {
	id := Gen()
	if id.IsZero() {
		t.Fatal("generated id should not be zero")
	}

	s := id.String()
	if len(s) != 32 {
		t.Fatalf("unexpected string length: %d", len(s))
	}

	parsed, err := ParseString(s)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed != id {
		t.Fatalf("round trip mismatch: got %s want %s", parsed, id)
	}
}

func TestParseStringWithDashes(t *testing.T) {
	id := Gen()
	dashed := id.String()[:8] + "-" + id.String()[8:12] + "-" + id.String()[12:16] + "-" + id.String()[16:20] + "-" + id.String()[20:]

	parsed, err := ParseString(dashed)
	if err != nil {
		t.Fatalf("parse dashed failed: %v", err)
	}
	if parsed != id {
		t.Fatalf("parse dashed mismatch: got %s want %s", parsed, id)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := ParseString("not-a-uuid"); err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestZero(t *testing.T) {
	var id ID
	if !id.IsZero() {
		t.Fatal("zero id should be zero")
	}
	if s := id.String(); s != "" {
		t.Fatalf("zero id string should be empty, got %q", s)
	}
}

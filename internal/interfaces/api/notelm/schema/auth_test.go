package schema

import "testing"

func TestIsSafeReturnTo(t *testing.T) {
	allowed := []string{"https://app.example.com", "http://localhost:5173"}

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty", "", true},
		{"relative", "/dashboard", true},
		{"relative with query", "/a/b?x=1", true},
		{"protocol relative", "//evil.com", false},
		{"backslash", "/\\evil.com", false},
		{"absolute allowed", "https://app.example.com/callback", true},
		{"absolute allowed with port", "http://localhost:5173/x", true},
		{"absolute disallowed", "https://evil.com", false},
		{"scheme relative", "https:/evil.com", false},
		{"javascript", "javascript:alert(1)", false},
		{"crlf", "/a\r\nSet-Cookie:x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSafeReturnTo(tc.raw, allowed); got != tc.want {
				t.Fatalf("IsSafeReturnTo(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

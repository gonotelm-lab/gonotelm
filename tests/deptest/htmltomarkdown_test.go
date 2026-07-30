package deptest

import (
	"net/http"
	"strings"
	"testing"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

func TestHtmlToMarkdown_NonHtmlInput(t *testing.T) {
	inputs := []struct {
		name string
		data string
	}{
		{"plain_text", "Hello, 世界！This is plain text."},
		{"json", `{"name": "test", "value": 123}`},
		{"xml", "<root><item>hello</item></root>"},
		{"empty", ""},
		{"markdown itself", "# Hello\n\nThis is **markdown**"},
	}

	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			md, err := htmltomarkdown.ConvertReader(
				strings.NewReader(in.data),
			)
			if err != nil {
				t.Logf("ERROR: %v", err)
				return
			}
			body := string(md)
			t.Logf("Input: %q", in.data)
			t.Logf("Output:\n%s", body)
		})
	}
}

func TestHtmlToMarkdown_RealPage(t *testing.T) {
	url := "https://baike.baidu.com/item/Go%E8%AF%AD%E8%A8%80"

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("fetch %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	md, err := htmltomarkdown.ConvertReader(
		resp.Body,
		converter.WithDomain(url),
	)
	if err != nil {
		t.Fatalf("ConvertReader failed: %v", err)
	}

	body := string(md)
	t.Logf("Total length: %d bytes", len(body))

	if len(body) > 3000 {
		t.Logf("First 3000 chars:\n%s\n\n...(truncated, %d more bytes)...", body[:3000], len(body)-3000)
	} else {
		t.Logf("Body:\n%s", body)
	}
}

package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

func TestEPUBParser_Parse(t *testing.T) {
	epubData := buildTestEPUB(t)

	docs, err := NewEPUBParser(&Config{OutputFormat: OutputFormatHTML}).Parse(
		context.Background(),
		bytes.NewReader(epubData),
		einoparser.WithURI("book.epub"),
		einoparser.WithExtraMeta(map[string]any{"origin": "unit-test"}),
	)
	if err != nil {
		t.Fatalf("parse epub failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expect one document, got %d", len(docs))
	}

	content := docs[0].Content
	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Fatalf("expect html doctype in parsed content, got: %s", content)
	}
	if !strings.Contains(content, "Chapter One") || !strings.Contains(content, "Chapter Two") {
		t.Fatalf("expect both chapters in parsed content, got: %s", content)
	}
	if !strings.Contains(content, "data-epub-index=\"1\"") || !strings.Contains(content, "data-epub-index=\"2\"") {
		t.Fatalf("expect chapter sections in parsed content, got: %s", content)
	}

	if docs[0].MetaData["content_type"] != "text/html" {
		t.Fatalf("expect content_type text/html, got: %#v", docs[0].MetaData["content_type"])
	}
	if docs[0].MetaData[einoparser.MetaKeySource] != "book.epub" {
		t.Fatalf("expect source metadata book.epub, got: %#v", docs[0].MetaData[einoparser.MetaKeySource])
	}
	if docs[0].MetaData["origin"] != "unit-test" {
		t.Fatalf("expect custom metadata preserved, got: %#v", docs[0].MetaData["origin"])
	}
}

func TestEPUBParser_ParseAsMarkdown(t *testing.T) {
	epubData := buildTestEPUB(t)

	docs, err := NewEPUBParser(&Config{OutputFormat: OutputFormatMarkdown}).Parse(
		context.Background(),
		bytes.NewReader(epubData),
	)
	if err != nil {
		t.Fatalf("parse epub as markdown failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expect one document, got %d", len(docs))
	}

	content := docs[0].Content
	if strings.Contains(content, "<html") || strings.Contains(content, "<section") {
		t.Fatalf("expect markdown output without html tags, got: %s", content)
	}
	if !strings.Contains(content, "Chapter One") || !strings.Contains(content, "Chapter Two") {
		t.Fatalf("expect converted markdown keeps chapter content, got: %s", content)
	}
	if docs[0].MetaData["content_type"] != "text/markdown" {
		t.Fatalf("expect content_type text/markdown, got: %#v", docs[0].MetaData["content_type"])
	}
}

func TestEPUBParser_ParseToPages(t *testing.T) {
	epubData := buildTestEPUB(t)

	docs, err := NewEPUBParser(&Config{
		OutputFormat: OutputFormatHTML,
		ToPages:      true,
	}).Parse(context.Background(), bytes.NewReader(epubData))
	if err != nil {
		t.Fatalf("parse epub to pages failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expect two documents when to_pages is enabled, got %d", len(docs))
	}

	if !strings.Contains(docs[0].Content, "Chapter One") || strings.Contains(docs[0].Content, "Chapter Two") {
		t.Fatalf("expect first page only contains chapter one, got: %s", docs[0].Content)
	}
	if docs[0].MetaData["epub_section_index"] != 1 {
		t.Fatalf("expect first page section index 1, got: %#v", docs[0].MetaData["epub_section_index"])
	}
	if docs[0].MetaData["content_type"] != "text/html" {
		t.Fatalf("expect first page content_type text/html, got: %#v", docs[0].MetaData["content_type"])
	}

	if !strings.Contains(docs[1].Content, "Chapter Two") || strings.Contains(docs[1].Content, "Chapter One") {
		t.Fatalf("expect second page only contains chapter two, got: %s", docs[1].Content)
	}
	if docs[1].MetaData["epub_section_index"] != 2 {
		t.Fatalf("expect second page section index 2, got: %#v", docs[1].MetaData["epub_section_index"])
	}
}

func TestEPUBParser_ParseToPagesAsMarkdown(t *testing.T) {
	epubData := buildTestEPUB(t)

	docs, err := NewEPUBParser(&Config{
		OutputFormat: OutputFormatMarkdown,
		ToPages:      true,
	}).Parse(context.Background(), bytes.NewReader(epubData))
	if err != nil {
		t.Fatalf("parse epub markdown to pages failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expect two documents when markdown to_pages is enabled, got %d", len(docs))
	}
	if docs[0].MetaData["content_type"] != "text/markdown" {
		t.Fatalf("expect markdown content_type on first page, got: %#v", docs[0].MetaData["content_type"])
	}
	if strings.Contains(docs[0].Content, "<html") {
		t.Fatalf("expect markdown content without html tag, got: %s", docs[0].Content)
	}
}

func buildTestEPUB(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	addZipFile := func(name string, content string) {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", name, err)
		}
	}

	addZipFile("mimetype", "application/epub+zip")
	addZipFile("META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)
	addZipFile("OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <manifest>
    <item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`)
	addZipFile("OEBPS/chapter1.xhtml", `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter One</title></head>
  <body><h1>Chapter One</h1><p>Hello EPUB.</p></body>
</html>`)
	addZipFile("OEBPS/chapter2.xhtml", `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Chapter Two</title></head>
  <body><h1>Chapter Two</h1><p>Second chapter.</p></body>
</html>`)

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}

	return buf.Bytes()
}

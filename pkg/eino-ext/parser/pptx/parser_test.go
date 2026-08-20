package pptx

import (
	"bytes"
	"context"
	"strings"
	"testing"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

func TestPptxParser_EndToEnd(t *testing.T) {
	slide1 := slideXML(
		spText("Title 1", "10", "20", "title", p(0, r("Meeting Notes"))) +
			spText("Body 1", "10", "300", "body", p(0, r("Agenda"))+p(1, r("Intro"))+p(2, r("Details"))),
	)
	slide2 := slideXML(
		spTable([]string{"Name", "Qty"}, []string{"Apple", "3"}, []string{"Pipe | bar", "2"}) +
			spPic("My Picture 1", "A red circle", "200", "400"),
	)
	notes2 := notesSlideXML(spText("Notes Placeholder", "", "", "body", p(0, r("Remember to"))+p(0, r("follow up"))))
	slide3 := slideXML(spText("T", "", "", "title", p(0, r("")))) // 空标题形状 + 无备注

	data := buildDeck(t, []string{slide1, slide2, slide3}, []string{"", notes2, ""})

	docs, err := NewParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs)=%d want 1", len(docs))
	}
	content := docs[0].Content
	t.Logf("content:\n%s", content)

	for _, want := range []string{
		"<!-- Slide number: 1 -->",
		"# Meeting Notes",
		"Agenda",
		"- Intro",
		"  - Details",
		"<!-- Slide number: 2 -->",
		"| Name | Qty |",
		"| Apple | 3 |",
		`| Pipe \| bar | 2 |`,
		"![A red circle](MyPicture1.jpg)",
		"### Notes:",
		"Remember to\nfollow up",
		"<!-- Slide number: 3 -->",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in content:\n%s", want, content)
		}
	}

	if docs[0].MetaData["content_type"] != "text/markdown" {
		t.Fatalf("content_type=%v", docs[0].MetaData["content_type"])
	}
	if docs[0].ID == "" {
		t.Fatal("expected document ID")
	}
}

func TestPptxParser_MetaURIAndExtra(t *testing.T) {
	slide := slideXML(spText("T", "", "", "title", p(0, r("Hello"))))
	data := buildDeck(t, []string{slide}, nil)

	docs, err := NewParser(nil).Parse(context.Background(), bytes.NewReader(data),
		einoparser.WithURI("deck.pptx"),
		einoparser.WithExtraMeta(map[string]any{"origin": "unit-test"}),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	meta := docs[0].MetaData
	if meta[einoparser.MetaKeySource] != "deck.pptx" {
		t.Fatalf("source=%v", meta[einoparser.MetaKeySource])
	}
	if meta["origin"] != "unit-test" {
		t.Fatalf("origin=%v", meta["origin"])
	}
}

func TestPptxParser_InvalidZip(t *testing.T) {
	if _, err := NewParser(nil).Parse(context.Background(), bytes.NewReader([]byte("garbage"))); err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestPptxParser_EmptyPresentation(t *testing.T) {
	data := buildTestPPTX(t,
		zipEntry{"[Content_Types].xml", contentTypesXML},
		zipEntry{"_rels/.rels", rootRelsXML},
		zipEntry{"ppt/presentation.xml", presentationXML(0)},
		zipEntry{"ppt/_rels/presentation.xml.rels", presentationRelsXML(0)},
	)
	if _, err := NewParser(nil).Parse(context.Background(), bytes.NewReader(data)); err == nil {
		t.Fatal("expected error for empty presentation")
	}
}

func TestPptxParser_RealWorldFile(t *testing.T) {
	// 真实结构的 pptx：带 sldIdLst、bodyPr/lstStyle、非表格 graphicFrame、ph idx 等。
	realSlide := `<p:sld ` + slideXMLNS + `><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="Title 1"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Real &amp; Demo</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="Content Placeholder 2"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>First bullet</a:t></a:r></a:p><a:p><a:pPr lvl="1"/><a:r><a:t>Second level</a:t></a:r></a:p><a:p><a:pPr/><a:r><a:t>Top again</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`

	data := buildDeck(t, []string{realSlide}, nil)
	docs, err := NewParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := docs[0].Content
	for _, want := range []string{
		"# Real & Demo",
		"First bullet",
		"- Second level",
		"Top again",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in content:\n%s", want, content)
		}
	}
}

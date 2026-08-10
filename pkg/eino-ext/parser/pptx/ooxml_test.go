package pptx

import (
	"context"
	"strings"
	"testing"
)

func mustParse(t *testing.T, data []byte) *pptxPresentation {
	t.Helper()
	pres, err := parsePresentation(context.Background(), data)
	if err != nil {
		t.Fatalf("parsePresentation: %v", err)
	}
	return pres
}

func TestParsePresentation_InvalidZip(t *testing.T) {
	if _, err := parsePresentation(context.Background(), []byte("not a zip")); err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestParsePresentation_NoSlides(t *testing.T) {
	data := buildTestPPTX(t,
		zipEntry{"ppt/presentation.xml", presentationXML(0)},
		zipEntry{"ppt/_rels/presentation.xml.rels", presentationRelsXML(0)},
	)
	if _, err := parsePresentation(context.Background(), data); err == nil {
		t.Fatal("expected error for empty presentation")
	}
}

func TestParseSlide_TextTitleAndLevels(t *testing.T) {
	slide := slideXML(
		spText("Title 1", "10", "20", "title", p(0, r("Hello"))+p(0, r("World"))) +
			spText("Body 1", "10", "300", "body", p(0, r("Top"))+p(1, r("Level1"))+p(2, r("Level2"))+p(0, r(""))),
	)
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	if len(pres.slides) != 1 {
		t.Fatalf("len(slides)=%d want 1", len(pres.slides))
	}
	shapes := pres.slides[0].shapes
	if len(shapes) != 2 {
		t.Fatalf("len(shapes)=%d want 2", len(shapes))
	}

	title := shapes[0]
	if title.phType != "title" || title.name != "Title 1" {
		t.Fatalf("title shape = %+v", title)
	}
	if len(title.paragraphs) != 2 || title.paragraphs[0].text != "Hello" {
		t.Fatalf("title paragraphs = %+v", title.paragraphs)
	}

	body := shapes[1]
	if body.phType != "body" {
		t.Fatalf("body phType=%q", body.phType)
	}
	want := []struct {
		level int
		text  string
	}{
		{0, "Top"},
		{1, "Level1"},
		{2, "Level2"},
		{0, ""},
	}
	if len(body.paragraphs) != len(want) {
		t.Fatalf("len(paragraphs)=%d want %d", len(body.paragraphs), len(want))
	}
	for i, w := range want {
		if body.paragraphs[i].level != w.level || body.paragraphs[i].text != w.text {
			t.Fatalf("paragraph[%d]=%+v want level=%d text=%q", i, body.paragraphs[i], w.level, w.text)
		}
	}
}

func TestParseSlide_RunBreakFieldTab(t *testing.T) {
	slide := slideXML(spText("Body 1", "", "", "", p(0, r("a")+`<a:br/>`+r("b"))+p(0, r("x")+`<a:tab/>`+r("y"))+p(0, `<a:fld id="1"><a:t>7</a:t></a:fld>`)))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	paras := pres.slides[0].shapes[0].paragraphs
	want := []string{"a\nb", "x\ty", "7"}
	for i, w := range want {
		if paras[i].text != w {
			t.Fatalf("paragraph[%d]=%q want %q", i, paras[i].text, w)
		}
	}
}

func TestParseSlide_PictureAlt(t *testing.T) {
	slide := slideXML(spPic("Pic 1", "A red circle", "100", "200"))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	pic := pres.slides[0].shapes[0]
	if pic.kind != shapeKindPicture || pic.name != "Pic 1" || pic.alt != "A red circle" {
		t.Fatalf("picture shape = %+v", pic)
	}
	if !pic.hasPos || pic.left != 100 || pic.top != 200 {
		t.Fatalf("picture position = %+v", pic)
	}
}

func TestParseSlide_Table(t *testing.T) {
	slide := slideXML(spTable([]string{"Name", "Qty"}, []string{"Apple", "3"}, []string{"Pipe | bar", "2"}))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	table := pres.slides[0].shapes[0]
	if table.kind != shapeKindTable {
		t.Fatalf("kind=%v want table", table.kind)
	}
	if len(table.rows) != 3 {
		t.Fatalf("len(rows)=%d want 3", len(table.rows))
	}
	if table.rows[0][0] != "Name" || table.rows[2][0] != "Pipe | bar" || table.rows[1][1] != "3" {
		t.Fatalf("rows = %+v", table.rows)
	}
}

func TestParseSlide_NonTableGraphicFrameSkipped(t *testing.T) {
	graphicFrame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="9" name="Chart 1"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId1"/></a:graphicData></a:graphic></p:graphicFrame>`
	slide := slideXML(spText("T", "", "", "title", p(0, r("Title"))) + graphicFrame)
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	shapes := pres.slides[0].shapes
	if len(shapes) != 1 || shapes[0].kind != shapeKindText {
		t.Fatalf("shapes=%+v want only text shape", shapes)
	}
}

func TestParseSlide_GroupRecursion(t *testing.T) {
	inner := spText("G Text", "10", "10", "", p(0, r("Grouped")))
	nested := spGroup("0", "0", spText("Nested", "0", "0", "", p(0, r("Deep"))))
	slide := slideXML(spGroup("100", "50", inner+nested))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	group := pres.slides[0].shapes[0]
	if group.kind != shapeKindGroup {
		t.Fatalf("kind=%v want group", group.kind)
	}
	if !group.hasPos || group.left != 100 || group.top != 50 {
		t.Fatalf("group position = %+v", group)
	}
	if len(group.children) != 2 {
		t.Fatalf("len(children)=%d want 2", len(group.children))
	}
	if group.children[0].kind != shapeKindText || group.children[0].paragraphs[0].text != "Grouped" {
		t.Fatalf("child[0] = %+v", group.children[0])
	}
	nestedGroup := group.children[1]
	if nestedGroup.kind != shapeKindGroup || len(nestedGroup.children) != 1 {
		t.Fatalf("child[1] = %+v", nestedGroup)
	}
	if nestedGroup.children[0].paragraphs[0].text != "Deep" {
		t.Fatalf("nested text = %+v", nestedGroup.children[0].paragraphs)
	}
}

func TestParseSlide_UnknownShapesSkipped(t *testing.T) {
	connector := `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="8" name="Connector 1"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr><p:spPr/></p:cxnSp>`
	slide := slideXML(connector + spText("T", "", "", "title", p(0, r("Title"))))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	if len(pres.slides[0].shapes) != 1 {
		t.Fatalf("shapes=%+v want only text", pres.slides[0].shapes)
	}
}

func TestParseNotes_BodyPlaceholderOnly(t *testing.T) {
	notes := notesSlideXML(
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image"/><p:cNvSpPr/><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>` +
			spText("Notes Placeholder", "", "", "body", p(0, r("Line one"))+p(0, r("Line two"))) +
			`<p:sp><p:nvSpPr><p:cNvPr id="4" name="Slide Number"/><p:cNvSpPr/><p:nvPr><p:ph type="sldNum"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>`,
	)
	slide := slideXML(spText("T", "", "", "title", p(0, r("Slide one"))))
	data := buildDeck(t, []string{slide}, []string{notes})
	pres := mustParse(t, data)

	got := pres.slides[0].notes
	if got != "Line one\nLine two" {
		t.Fatalf("notes=%q", got)
	}
}

func TestParseNotes_MissingRelsNoNotes(t *testing.T) {
	slide := slideXML(spText("T", "", "", "title", p(0, r("Slide one"))))
	data := buildDeck(t, []string{slide}, nil)
	pres := mustParse(t, data)

	if pres.slides[0].notes != "" {
		t.Fatalf("notes=%q want empty", pres.slides[0].notes)
	}
}

func TestParseSlideOrder(t *testing.T) {
	s1 := slideXML(spText("T", "", "", "title", p(0, r("First"))))
	s2 := slideXML(spText("T", "", "", "title", p(0, r("Second"))))
	s3 := slideXML(spText("T", "", "", "title", p(0, r("Third"))))
	data := buildDeck(t, []string{s1, s2, s3}, nil)
	pres := mustParse(t, data)

	if len(pres.slides) != 3 {
		t.Fatalf("len(slides)=%d want 3", len(pres.slides))
	}
	var texts []string
	for _, s := range pres.slides {
		texts = append(texts, s.shapes[0].paragraphs[0].text)
	}
	if strings.Join(texts, ",") != "First,Second,Third" {
		t.Fatalf("slide order=%v", texts)
	}
}

func TestParseContextCancellation(t *testing.T) {
	slides := make([]string, 5)
	for i := range slides {
		slides[i] = slideXML(spText("T", "", "", "title", p(0, r("Slide"))))
	}
	data := buildDeck(t, slides, nil)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parsePresentation(cancelled, data); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

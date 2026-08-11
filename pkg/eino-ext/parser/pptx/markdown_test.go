package pptx

import (
	"context"
	"strings"
	"testing"
)

func TestRenderTextBlock_Title(t *testing.T) {
	sh := pptxShape{
		kind:   shapeKindText,
		phType: "title",
		paragraphs: []pptxParagraph{
			{level: 0, text: "Hello"},
			{level: 0, text: "Second line"},
		},
	}
	got := renderTextBlock(sh)
	if got != "# Hello" {
		t.Fatalf("renderTextBlock=%q want %q", got, "# Hello")
	}
}

func TestRenderTextBlock_CenteredTitle(t *testing.T) {
	sh := pptxShape{
		kind:   shapeKindText,
		phType: "ctrTitle",
		paragraphs: []pptxParagraph{
			{level: 0, text: "Centered"},
		},
	}
	if got := renderTextBlock(sh); got != "# Centered" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderTextBlock_BulletsByLevel(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindText,
		paragraphs: []pptxParagraph{
			{level: 0, text: "Top"},
			{level: 1, text: "One"},
			{level: 2, text: "Two"},
			{level: 3, text: "Three"},
			{level: 0, text: ""},
		},
	}
	got := renderTextBlock(sh)
	want := "Top\n- One\n  - Two\n    - Three"
	if got != want {
		t.Fatalf("renderTextBlock:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderTextBlock_BulletFlattensLineBreak(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindText,
		paragraphs: []pptxParagraph{
			{level: 1, text: "line1\nline2"},
		},
	}
	if got := renderTextBlock(sh); got != "- line1 line2" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderTextBlock_Empty(t *testing.T) {
	sh := pptxShape{kind: shapeKindText}
	if got := renderTextBlock(sh); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestRenderTableBlock(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindTable,
		rows: [][]string{
			{"Name", "Qty"},
			{"Apple", "3"},
			{"Pipe | bar", "line1\nline2"},
		},
	}
	got := renderTableBlock(sh)
	want := "| Name | Qty |\n| --- | --- |\n| Apple | 3 |\n| Pipe \\| bar | line1 line2 |"
	if got != want {
		t.Fatalf("renderTableBlock:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTableBlock_RaggedRows(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindTable,
		rows: [][]string{
			{"A", "B"},
			{"only-one"},
		},
	}
	got := renderTableBlock(sh)
	want := "| A | B |\n| --- | --- |\n| only-one |  |"
	if got != want {
		t.Fatalf("renderTableBlock:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTableBlock_Empty(t *testing.T) {
	sh := pptxShape{kind: shapeKindTable, rows: [][]string{{""}}}
	if got := renderTableBlock(sh); got != "|  |\n| --- |" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderPictureBlock(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindPicture,
		name: "My Picture 1",
		alt:  "A [red] circle\nwith  spaces",
	}
	got := renderPictureBlock(sh)
	if got != "![A red circle with spaces](MyPicture1.jpg)" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderPictureBlock_AltFallbackToName(t *testing.T) {
	sh := pptxShape{kind: shapeKindPicture, name: "Picture 2"}
	if got := renderPictureBlock(sh); got != "![Picture 2](Picture2.jpg)" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderPictureBlock_EmptyName(t *testing.T) {
	sh := pptxShape{kind: shapeKindPicture, name: "中文图片"}
	if got := renderPictureBlock(sh); got != "![中文图片](image.jpg)" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"Hello World!": "HelloWorld",
		"a_b-1":        "a_b1",
		"":             "image",
		"   ":          "image",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Fatalf("sanitizeFilename(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSortShapes(t *testing.T) {
	missing := func(name string) pptxShape {
		return pptxShape{name: name}
	}
	pos := func(name string, top, left int64) pptxShape {
		return pptxShape{name: name, hasPos: true, top: top, left: left}
	}
	shapes := []pptxShape{
		pos("bottom", 300, 0),
		missing("no-pos"),
		pos("top", 10, 999),
		pos("left-of-top", 10, 5),
	}
	sorted := sortShapes(shapes)
	var names []string
	for _, s := range sorted {
		names = append(names, s.name)
	}
	want := "no-pos,left-of-top,top,bottom"
	if strings.Join(names, ",") != want {
		t.Fatalf("sorted=%v want %s", names, want)
	}
}

func TestRenderGroupBlock(t *testing.T) {
	sh := pptxShape{
		kind: shapeKindGroup,
		children: []pptxShape{
			{kind: shapeKindText, paragraphs: []pptxParagraph{{level: 0, text: "First"}}},
			{kind: shapeKindText, paragraphs: []pptxParagraph{{level: 0, text: "Second"}}},
			{kind: shapeKindText, paragraphs: []pptxParagraph{{level: 0, text: ""}}},
		},
	}
	got := renderShapeBlock(sh)
	if got != "First\n\nSecond" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderSlideMarkdown_Notes(t *testing.T) {
	slide := pptxSlide{
		index: 3,
		shapes: []pptxShape{
			{kind: shapeKindText, phType: "title", paragraphs: []pptxParagraph{{level: 0, text: "T"}}},
		},
		notes: "  speaker note  ",
	}
	got := renderSlideMarkdown(3, slide)
	want := "<!-- Slide number: 3 -->\n\n# T\n\n### Notes:\nspeaker note"
	if got != want {
		t.Fatalf("renderSlideMarkdown:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderSlideMarkdown_EmptySlide(t *testing.T) {
	slide := pptxSlide{index: 1}
	if got := renderSlideMarkdown(1, slide); got != "<!-- Slide number: 1 -->\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderPresentationMarkdown_JoinsSlides(t *testing.T) {
	pres := &pptxPresentation{slides: []pptxSlide{
		{index: 1, shapes: []pptxShape{{kind: shapeKindText, phType: "title", paragraphs: []pptxParagraph{{text: "A"}}}}},
		{index: 2, shapes: []pptxShape{{kind: shapeKindText, paragraphs: []pptxParagraph{{text: "B"}}}}},
	}}
	got, err := renderPresentationMarkdown(context.Background(), pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "<!-- Slide number: 1 -->\n\n# A\n\n<!-- Slide number: 2 -->\n\nB"
	if got != want {
		t.Fatalf("render:\n%q\nwant:\n%q", got, want)
	}
}

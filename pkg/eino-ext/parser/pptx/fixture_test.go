package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// zipEntry 描述测试 pptx 中的一个文件。
type zipEntry struct {
	name string
	body string
}

// buildTestPPTX 将条目打包为内存中的 pptx（zip）字节流。
func buildTestPPTX(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatalf("write zip entry %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

const slideXMLNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

// presentationXML 生成按 slides 数量排列 sldId 的 presentation.xml。
func presentationXML(n int) string {
	var b strings.Builder
	b.WriteString(`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>`)
	for i := range n {
		fmt.Fprintf(&b, `<p:sldId id="%d" r:id="rId%d"/>`, 256+i, 2+i)
	}
	b.WriteString(`</p:sldIdLst></p:presentation>`)
	return b.String()
}

// presentationRelsXML 生成 rId2..n -> slides/slideN.xml 的 presentation rels。
func presentationRelsXML(n int) string {
	var b strings.Builder
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, 2+i, i+1)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// slideRelsXML 生成指向 notesSlideN 的 slide rels。
func slideRelsXML(notesIndex int) string {
	return fmt.Sprintf(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide%d.xml"/>
</Relationships>`, notesIndex)
}

// buildDeck 构造包含指定幻灯片（及可选备注）的完整 pptx 字节流。
// notes 与 slides 一一对应，空串表示该页无备注。
func buildDeck(t *testing.T, slides []string, notes []string) []byte {
	t.Helper()
	entries := []zipEntry{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", rootRelsXML},
		{"ppt/presentation.xml", presentationXML(len(slides))},
		{"ppt/_rels/presentation.xml.rels", presentationRelsXML(len(slides))},
	}
	for i, s := range slides {
		entries = append(entries, zipEntry{
			name: fmt.Sprintf("ppt/slides/slide%d.xml", i+1),
			body: s,
		})
		if i < len(notes) && notes[i] != "" {
			entries = append(entries,
				zipEntry{
					name: fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1),
					body: slideRelsXML(i + 1),
				},
				zipEntry{
					name: fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", i+1),
					body: notes[i],
				},
			)
		}
	}
	return buildTestPPTX(t, entries...)
}

// slideXML 包装 spTree 内容为完整 slide 文档。
func slideXML(spTree string) string {
	return `<p:sld ` + slideXMLNS + `><p:cSld><p:spTree>` + spTree + `</p:spTree></p:cSld></p:sld>`
}

// notesSlideXML 包装 spTree 内容为完整 notes 文档。
func notesSlideXML(spTree string) string {
	return `<p:notes ` + slideXMLNS + `><p:cSld><p:spTree>` + spTree + `</p:spTree></p:cSld></p:notes>`
}

// spText 构造一个文本形状。x/y 为空表示无位置；phType 为空表示非占位符。
func spText(name, x, y, phType string, paragraphs string) string {
	var xfrm, ph string
	if x != "" && y != "" {
		xfrm = fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="%s" y="%s"/></a:xfrm></p:spPr>`, x, y)
	} else {
		xfrm = `<p:spPr/>`
	}
	if phType != "" {
		ph = fmt.Sprintf(`<p:ph type="%s"/>`, phType)
	}
	return `<p:sp><p:nvSpPr><p:cNvPr id="1" name="` + name + `" descr=""/><p:cNvSpPr/><p:nvPr>` + ph + `</p:nvPr></p:nvSpPr>` + xfrm + `<p:txBody><a:bodyPr/>` + paragraphs + `</p:txBody></p:sp>`
}

// spPic 构造一个图片形状，alt 为 descr。
func spPic(name, alt, x, y string) string {
	var xfrm string
	if x != "" && y != "" {
		xfrm = fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="%s" y="%s"/></a:xfrm></p:spPr>`, x, y)
	} else {
		xfrm = `<p:spPr/>`
	}
	return `<p:pic><p:nvPicPr><p:cNvPr id="2" name="` + name + `" descr="` + alt + `"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId9"/></p:blipFill>` + xfrm + `</p:pic>`
}

// spGroup 构造组形状，children 为组内形状 XML。
func spGroup(x, y string, children string) string {
	var xfrm string
	if x != "" && y != "" {
		xfrm = fmt.Sprintf(`<a:xfrm><a:off x="%s" y="%s"/></a:xfrm>`, x, y)
	}
	return `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="3" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr>` + xfrm + `</p:grpSpPr>` + children + `</p:grpSp>`
}

// p 构造一个段落。lvl 为 0 表示无 lvl 属性。
func p(lvl int, body string) string {
	pPr := ""
	if lvl > 0 {
		pPr = fmt.Sprintf(`<a:pPr lvl="%d"/>`, lvl)
	}
	return `<a:p>` + pPr + body + `</a:p>`
}

// r 构造一个文本 run。
func r(text string) string {
	return `<a:r><a:t>` + xmlEscape(text) + `</a:t></a:r>`
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// spTable 构造表格 graphicFrame。rows 为行，每行是单元格文本切片。
func spTable(rows ...[]string) string {
	var b strings.Builder
	b.WriteString(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="4" name="Table 1"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="1000" y="1000"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/><a:tblGrid/>`)
	for _, row := range rows {
		b.WriteString(`<a:tr>`)
		for _, cell := range row {
			b.WriteString(`<a:tc><a:txBody><a:bodyPr/>`)
			b.WriteString(p(0, r(cell)))
			b.WriteString(`</a:txBody></a:tc>`)
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return b.String()
}

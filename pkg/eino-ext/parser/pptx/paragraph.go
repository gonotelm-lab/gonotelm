package pptx

import (
	"encoding/xml"
	"io"
	"strings"
)

// rawParagraph 流式解析 a:p 元素：a:pPr（lvl）、a:r/a:fld（文本）、a:br（换行）、a:tab（制表符）。
type rawParagraph struct {
	PPr  *rawPPr
	Runs []rawRun
}

// UnmarshalXML 手动遍历 a:p 的子元素，避免 xml.Unmarshal 对 fragment
// 根元素的语义（fragment 的第一个元素会被当作整体解到结构体，导致兄弟元素丢失）。
func (p *rawParagraph) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				var pp rawPPr
				if err := d.DecodeElement(&pp, &t); err != nil {
					return err
				}
				p.PPr = &pp
			case "r", "fld":
				var run struct {
					T string `xml:"t"`
				}
				if err := d.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, rawRun{kind: runKindText, text: run.T})
			case "br":
				p.Runs = append(p.Runs, rawRun{kind: runKindBreak})
				if err := d.Skip(); err != nil {
					return err
				}
			case "tab":
				p.Runs = append(p.Runs, rawRun{kind: runKindTab})
				if err := d.Skip(); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

type rawPPr struct {
	Lvl *int `xml:"lvl,attr"`
}

func (p rawParagraph) level() int {
	if p.PPr != nil && p.PPr.Lvl != nil && *p.PPr.Lvl > 0 {
		return *p.PPr.Lvl
	}
	return 0
}

func (p rawParagraph) text() string {
	var b strings.Builder
	for _, r := range p.Runs {
		switch r.kind {
		case runKindText:
			b.WriteString(r.text)
		case runKindBreak:
			b.WriteString("\n")
		case runKindTab:
			b.WriteString("\t")
		}
	}
	return b.String()
}

type runKind int

const (
	runKindText runKind = iota
	runKindBreak
	runKindTab
)

type rawRun struct {
	kind runKind
	text string
}

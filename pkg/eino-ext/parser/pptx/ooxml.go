package pptx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// 压缩包内文件路径。
const (
	pathPresentation     = "ppt/presentation.xml"
	pathPresentationRels = "ppt/_rels/presentation.xml.rels"
	pathSlidesDir        = "ppt"
)

// 关系类型。
const (
	relTypeSlide = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	relTypeNotes = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
)

// 占位符类型（p:ph@type）。
const (
	phTypeTitle    = "title"
	phTypeCtrTitle = "ctrTitle"
	phTypeBody     = "body"
)

// pptxPresentation 是从 pptx 压缩包中解析出的内容模型：按顺序排列的幻灯片与每页备注。
type pptxPresentation struct {
	slides []pptxSlide
}

type pptxSlide struct {
	index int
	// shapes 为按文档序收集的顶层形状；渲染时再按位置排序。
	shapes []pptxShape
	// notes 为演讲者备注文本（notes 幻灯片 body 占位符文本），可能为空。
	notes string
}

// pptxShape 是 spTree 中一个有语义的形状（文本/图片/表格/组）。
type pptxShape struct {
	kind   shapeKind
	name   string // cNvPr@name
	alt    string // cNvPr@descr，图片 alt text
	left   int64
	top    int64
	hasPos bool // xfrm/off 是否存在
	// phType 为占位符类型（p:ph@type），title/ctrTitle 为标题。
	phType string
	// 文本形状的段落。
	paragraphs []pptxParagraph
	// 表格形状的行。
	rows [][]string
	// 组形状的子形状。
	children []pptxShape
}

type shapeKind int

const (
	shapeKindText shapeKind = iota
	shapeKindPicture
	shapeKindTable
	shapeKindGroup
)

type pptxParagraph struct {
	level int
	text  string
}

// parsePresentation 从 pptx 字节流中解析出幻灯片内容。
func parsePresentation(ctx context.Context, data []byte) (*pptxPresentation, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pptx open zip: %w", err)
	}
	z := &pptxZip{reader: zr}

	presXML, err := z.readFile(pathPresentation)
	if err != nil {
		return nil, fmt.Errorf("pptx read %s: %w", pathPresentation, err)
	}
	presRels, err := z.readFile(pathPresentationRels)
	if err != nil {
		return nil, fmt.Errorf("pptx read %s: %w", pathPresentationRels, err)
	}

	slidePaths, err := resolveSlideOrder(presXML, presRels)
	if err != nil {
		return nil, err
	}
	if len(slidePaths) == 0 {
		return nil, fmt.Errorf("pptx no slides found")
	}

	pres := &pptxPresentation{slides: make([]pptxSlide, 0, len(slidePaths))}
	for idx, slidePath := range slidePaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		slide, err := parseSlide(z, slidePath, idx+1)
		if err != nil {
			return nil, err
		}
		pres.slides = append(pres.slides, *slide)
	}
	return pres, nil
}

// pptxZip 封装 zip 读取器，按路径读取文件。
type pptxZip struct {
	reader *zip.Reader
}

func (z *pptxZip) readFile(name string) ([]byte, error) {
	for _, f := range z.reader.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("pptx file %q not found", name)
}

// presentationDoc 匹配 ppt/presentation.xml 的 sldIdLst。
type presentationDoc struct {
	SldIDLst *sldIDList `xml:"sldIdLst"`
}

type sldIDList struct {
	SldIDs []sldID `xml:"sldId"`
}

type sldID struct {
	// r:id 属性带命名空间，必须用完整 URL 精确匹配，避免误匹配无命名空间的 id 属性。
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// resolveSlideOrder 解析 presentation.xml 中 sldIdLst 的文档顺序，
// 通过 presentation rels 映射 rId -> 幻灯片路径。
func resolveSlideOrder(presXML, presRels []byte) ([]string, error) {
	var pres presentationDoc
	if err := xml.Unmarshal(presXML, &pres); err != nil {
		return nil, fmt.Errorf("pptx decode presentation.xml: %w", err)
	}

	rels, err := parseRels(presRels)
	if err != nil {
		return nil, fmt.Errorf("pptx decode presentation rels: %w", err)
	}

	slides := make([]string, 0, len(pres.SldIDLst.SldIDs))
	for _, sldID := range pres.SldIDLst.SldIDs {
		rel, ok := rels[sldID.RID]
		if !ok || rel.Type != relTypeSlide {
			continue
		}
		slides = append(slides, path.Clean(path.Join(pathSlidesDir, rel.Target)))
	}
	return slides, nil
}

// relsDoc 匹配 .rels 文件。
type relsDoc struct {
	Relationships []relationship `xml:"Relationship"`
}

type relationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

// parseRels 解析 .rels 文件为 rId -> Relationship。
func parseRels(data []byte) (map[string]relationship, error) {
	var doc relsDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := make(map[string]relationship, len(doc.Relationships))
	for _, r := range doc.Relationships {
		out[r.ID] = r
	}
	return out, nil
}

// parseSlide 解析单张幻灯片及其备注。
func parseSlide(z *pptxZip, slidePath string, index int) (*pptxSlide, error) {
	slideXML, err := z.readFile(slidePath)
	if err != nil {
		return nil, fmt.Errorf("pptx read slide %d: %w", index, err)
	}

	shapes, err := parseSlideShapes(slideXML)
	if err != nil {
		return nil, fmt.Errorf("pptx parse slide %d: %w", index, err)
	}

	slide := &pptxSlide{
		index:  index,
		shapes: shapes,
	}

	// 备注：通过 slide rels 找到 notesSlide。
	baseDir := path.Dir(slidePath)
	relsPath := path.Join(baseDir, "_rels", path.Base(slidePath)+".rels")
	relsData, err := z.readFile(relsPath)
	if err != nil {
		return slide, nil // 无 rels 视为无备注
	}
	rels, err := parseRels(relsData)
	if err != nil {
		return slide, nil
	}
	for _, rel := range rels {
		if rel.Type != relTypeNotes {
			continue
		}
		notesPath := path.Clean(path.Join(baseDir, rel.Target))
		notesData, err := z.readFile(notesPath)
		if err != nil {
			continue
		}
		notes, err := parseNotesText(notesData)
		if err == nil {
			slide.notes = notes
		}
		break
	}

	return slide, nil
}

// parseSlideShapes 解析幻灯片 cSld/spTree 下的形状。
func parseSlideShapes(data []byte) ([]pptxShape, error) {
	var doc slideShapeDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.CSld.SpTree.shapes, nil
}

// parseNotesText 解析备注幻灯片，返回 body 占位符的文本。
func parseNotesText(notesXML []byte) (string, error) {
	var doc slideShapeDoc
	if err := xml.Unmarshal(notesXML, &doc); err != nil {
		return "", err
	}
	for _, sh := range doc.CSld.SpTree.shapes {
		if sh.kind != shapeKindText || sh.phType != phTypeBody {
			continue
		}
		var parts []string
		for _, p := range sh.paragraphs {
			parts = append(parts, p.text)
		}
		return strings.TrimSpace(strings.Join(parts, "\n")), nil
	}
	return "", nil
}

// slideShapeDoc 匹配 p:sld / p:notes 的 cSld/spTree 结构。
type slideShapeDoc struct {
	CSld struct {
		SpTree spTreeWalker `xml:"spTree"`
	} `xml:"cSld"`
}

// spTreeWalker 解码 spTree 元素时收集子形状。
type spTreeWalker struct {
	shapes []pptxShape
}

func (w *spTreeWalker) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return walkShapeChildren(d, &w.shapes)
}

// walkShapeChildren 遍历当前容器的直接子元素（解码器定位在容器 start 之后），
// 把有语义的形状追加到 out。
func walkShapeChildren(d *xml.Decoder, out *[]pptxShape) error {
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
			sh, err := parseShapeNode(d, t)
			if err != nil {
				return err
			}
			if sh != nil {
				*out = append(*out, *sh)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// parseShapeNode 根据元素名分发解析单个形状节点。
// 返回 (nil, nil) 表示该元素无语义（如连接符、OLE 对象），已跳过。
func parseShapeNode(d *xml.Decoder, start xml.StartElement) (*pptxShape, error) {
	switch start.Name.Local {
	case "sp":
		return parseTextShape(d, start)
	case "pic":
		return parsePictureShape(d, start)
	case "graphicFrame":
		return parseGraphicFrame(d, start)
	case "grpSp":
		sh := &pptxShape{kind: shapeKindGroup}
		if err := walkGroupChildren(d, sh); err != nil {
			return nil, err
		}
		return sh, nil
	default:
		if err := d.Skip(); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

func parseTextShape(d *xml.Decoder, start xml.StartElement) (*pptxShape, error) {
	var sp rawTextShape
	if err := d.DecodeElement(&sp, &start); err != nil {
		return nil, err
	}

	sh := &pptxShape{
		kind:       shapeKindText,
		name:       sp.NvSpPr.CNvPr.Name,
		alt:        sp.NvSpPr.CNvPr.Descr,
		paragraphs: sp.TxBody.paragraphs(),
	}
	if sp.NvSpPr.NvPr.Ph != nil {
		sh.phType = sp.NvSpPr.NvPr.Ph.Type
	}
	sp.SpPr.applyPosition(sh)
	return sh, nil
}

func parsePictureShape(d *xml.Decoder, start xml.StartElement) (*pptxShape, error) {
	var pic rawPicture
	if err := d.DecodeElement(&pic, &start); err != nil {
		return nil, err
	}

	sh := &pptxShape{
		kind: shapeKindPicture,
		name: pic.NvPicPr.CNvPr.Name,
		alt:  pic.NvPicPr.CNvPr.Descr,
	}
	pic.SpPr.applyPosition(sh)
	return sh, nil
}

func parseGraphicFrame(d *xml.Decoder, start xml.StartElement) (*pptxShape, error) {
	var gf rawGraphicFrame
	if err := d.DecodeElement(&gf, &start); err != nil {
		return nil, err
	}
	if gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.Tbl == nil {
		return nil, nil // 非表格 graphicFrame（如图表、OLE）无语义，丢弃
	}

	sh := &pptxShape{kind: shapeKindTable}
	if gf.Xfrm != nil && gf.Xfrm.Off != nil {
		sh.left = gf.Xfrm.Off.X
		sh.top = gf.Xfrm.Off.Y
		sh.hasPos = true
	}
	for _, tr := range gf.Graphic.GraphicData.Tbl.Rows {
		row := make([]string, 0, len(tr.Cells))
		for _, tc := range tr.Cells {
			row = append(row, tc.TxBody.paragraphsText())
		}
		sh.rows = append(sh.rows, row)
	}
	return sh, nil
}

// walkGroupChildren 遍历 grpSp 的子元素：nvGrpSpPr 直接跳过，
// grpSpPr 提取位置，其余（sp/pic/graphicFrame/嵌套 grpSp）按形状解析。
func walkGroupChildren(d *xml.Decoder, sh *pptxShape) error {
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
			if t.Name.Local == "grpSpPr" {
				var pr rawGroupProps
				if err := d.DecodeElement(&pr, &t); err != nil {
					return err
				}
				pr.applyPosition(sh)
				continue
			}
			child, err := parseShapeNode(d, t)
			if err != nil {
				return err
			}
			if child != nil {
				sh.children = append(sh.children, *child)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ---------- XML 结构类型 ----------

// rawCNvPr 通用形状属性（p:cNvPr）。
type rawCNvPr struct {
	ID    uint32 `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Descr string `xml:"descr,attr"`
}

// rawPh 占位符（p:nvPr/p:ph）。
type rawPh struct {
	Type string `xml:"type,attr"`
}

// rawNvPr 非可视形状属性（p:nvPr）。
type rawNvPr struct {
	Ph *rawPh `xml:"ph"`
}

// rawNvSpPr 文本形状的非可视属性（p:nvSpPr）。
type rawNvSpPr struct {
	CNvPr rawCNvPr `xml:"cNvPr"`
	NvPr  rawNvPr  `xml:"nvPr"`
}

// rawNvPicPr 图片形状的非可视属性（p:nvPicPr）。
type rawNvPicPr struct {
	CNvPr rawCNvPr `xml:"cNvPr"`
}

// rawOff 位置偏移（a:off）。
type rawOff struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// rawXfrm 形状变换（a:xfrm）。
type rawXfrm struct {
	Off *rawOff `xml:"off"`
}

func (x rawXfrm) applyPosition(sh *pptxShape) {
	if x.Off != nil {
		sh.left = x.Off.X
		sh.top = x.Off.Y
		sh.hasPos = true
	}
}

// rawShapePr 文本/图片形状属性（p:spPr），只关心位置。
type rawShapePr struct {
	Xfrm *rawXfrm `xml:"xfrm"`
}

func (p rawShapePr) applyPosition(sh *pptxShape) {
	if p.Xfrm != nil {
		p.Xfrm.applyPosition(sh)
	}
}

// rawGroupProps 组属性（p:grpSpPr），只关心位置。
type rawGroupProps struct {
	Xfrm *rawXfrm `xml:"xfrm"`
}

func (p rawGroupProps) applyPosition(sh *pptxShape) {
	if p.Xfrm != nil {
		p.Xfrm.applyPosition(sh)
	}
}

// rawTextShape 文本形状（p:sp）。
type rawTextShape struct {
	NvSpPr rawNvSpPr   `xml:"nvSpPr"`
	SpPr   rawShapePr  `xml:"spPr"`
	TxBody rawTextBody `xml:"txBody"`
}

// rawPicture 图片形状（p:pic）。
type rawPicture struct {
	NvPicPr rawNvPicPr `xml:"nvPicPr"`
	SpPr    rawShapePr `xml:"spPr"`
}

// rawGraphicFrame 图形框架（p:graphicFrame），只关心表格。
type rawGraphicFrame struct {
	Xfrm    *rawXfrm    `xml:"xfrm"`
	Graphic *rawGraphic `xml:"graphic"`
}

// rawGraphic 与 rawGraphicData 匹配 a:graphic/a:graphicData。
type rawGraphic struct {
	GraphicData *rawGraphicData `xml:"graphicData"`
}

type rawGraphicData struct {
	Tbl *rawTbl `xml:"tbl"`
}

// rawTbl 表格（a:tbl）。
type rawTbl struct {
	Rows []rawTr `xml:"tr"`
}

type rawTr struct {
	Cells []rawTc `xml:"tc"`
}

type rawTc struct {
	TxBody rawTextBody `xml:"txBody"`
}

// rawTextBody 文本体（a:txBody），段落解析见 paragraph.go。
type rawTextBody struct {
	Paragraphs []rawParagraph `xml:"p"`
}

func (b rawTextBody) paragraphs() []pptxParagraph {
	out := make([]pptxParagraph, 0, len(b.Paragraphs))
	for _, p := range b.Paragraphs {
		out = append(out, pptxParagraph{level: p.level(), text: p.text()})
	}
	return out
}

func (b rawTextBody) paragraphsText() string {
	var parts []string
	for _, p := range b.Paragraphs {
		parts = append(parts, p.text())
	}
	return strings.Join(parts, "\n")
}

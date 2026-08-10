package pptx

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// renderPresentationMarkdown 将解析出的演示文稿渲染为单个 markdown 字符串。
func renderPresentationMarkdown(ctx context.Context, pres *pptxPresentation) (string, error) {
	var parts []string
	for idx := range pres.slides {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		parts = append(parts, renderSlideMarkdown(idx+1, pres.slides[idx]))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

// renderSlideMarkdown 渲染单页幻灯片：
//
//	<!-- Slide number: N -->
//
//	# 标题
//	普通段落
//	- 列表项
//	![alt](file.jpg)
//	| 表格 |
//	...
//
//	### Notes:
//	备注文本
func renderSlideMarkdown(index int, slide pptxSlide) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- Slide number: %d -->\n", index)
	for _, sh := range sortShapes(slide.shapes) {
		if block := renderShapeBlock(sh); block != "" {
			b.WriteByte('\n')
			b.WriteString(block)
		}
	}
	if notes := strings.TrimSpace(slide.notes); notes != "" {
		b.WriteString("\n\n### Notes:\n")
		b.WriteString(notes)
	}
	return b.String()
}

// renderShapeBlock 渲染单个形状，返回不含首尾换行的 markdown 块；无语义返回空串。
func renderShapeBlock(sh pptxShape) string {
	switch sh.kind {
	case shapeKindText:
		return renderTextBlock(sh)
	case shapeKindTable:
		return renderTableBlock(sh)
	case shapeKindPicture:
		return renderPictureBlock(sh)
	case shapeKindGroup:
		var blocks []string
		for _, child := range sortShapes(sh.children) {
			if block := renderShapeBlock(child); block != "" {
				blocks = append(blocks, block)
			}
		}
		return strings.Join(blocks, "\n\n")
	}
	return ""
}

// renderTextBlock 渲染文本形状。标题占位符输出 "# " 前缀，
// lvl>0 的段落输出 "- " 列表项（按 lvl 缩进 2 空格）。
func renderTextBlock(sh pptxShape) string {
	isTitle := sh.phType == phTypeTitle || sh.phType == phTypeCtrTitle
	var lines []string
	for _, para := range sh.paragraphs {
		text := strings.TrimSpace(para.text)
		if text == "" {
			continue
		}
		if isTitle {
			if len(lines) == 0 {
				lines = append(lines, "# "+text)
			}
			continue
		}
		if para.level > 0 {
			text = strings.ReplaceAll(text, "\n", " ")
			lines = append(lines, strings.Repeat("  ", para.level-1)+"- "+text)
			continue
		}
		lines = append(lines, text)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// renderTableBlock 渲染表格：首行作表头并输出 "---" 分隔行，单元格内 "|" 转义。
func renderTableBlock(sh pptxShape) string {
	maxCols := 0
	for _, row := range sh.rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return ""
	}

	var lines []string
	for idx, row := range sh.rows {
		cells := make([]string, maxCols)
		for j := 0; j < maxCols; j++ {
			if j < len(row) {
				cells[j] = escapeCellText(row[j])
			}
		}
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
		if idx == 0 {
			sep := make([]string, maxCols)
			for j := range sep {
				sep[j] = "---"
			}
			lines = append(lines, "| "+strings.Join(sep, " | ")+" |")
		}
	}
	return strings.Join(lines, "\n")
}

// escapeCellText 压平单元格换行并转义表格管道符（与 xlsx parser 一致）。
func escapeCellText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

// renderPictureBlock 渲染图片为 ![alt](file.jpg)。alt 取 descr，
// 缺省回退 shape name；文件名取 shape name 的字母数字下划线部分。
func renderPictureBlock(sh pptxShape) string {
	alt := strings.TrimSpace(sh.alt)
	if alt == "" {
		alt = strings.TrimSpace(sh.name)
	}
	if alt == "" {
		alt = "Picture"
	}
	alt = collapseAltText(alt)
	filename := sanitizeFilename(sh.name)
	return "![" + alt + "](" + filename + ".jpg)"
}

// collapseAltText 移除 alt 中的换行与方括号，并压缩连续空白。
func collapseAltText(s string) string {
	s = strings.NewReplacer("[", " ", "]", " ", "\n", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// sanitizeFilename 仅保留字母、数字、下划线；结果为空时回退 "image"。
func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "image"
	}
	return b.String()
}

// sortShapes 按 (top, left) 排序；缺失位置的形状排最前。
func sortShapes(shapes []pptxShape) []pptxShape {
	sorted := append([]pptxShape(nil), shapes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ki, kj := shapeSortKey(sorted[i]), shapeSortKey(sorted[j])
		if ki[0] != kj[0] {
			return ki[0] < kj[0]
		}
		return ki[1] < kj[1]
	})
	return sorted
}

func shapeSortKey(sh pptxShape) [2]int64 {
	const missing = -1 << 62
	if !sh.hasPos {
		return [2]int64{missing, missing}
	}
	return [2]int64{sh.top, sh.left}
}

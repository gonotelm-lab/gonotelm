package pdf

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

var (
	partialNumberingPattern    = regexp.MustCompile(`^\.\d+$`)
	codeKeywordPattern         = regexp.MustCompile(`(?i)\b(func|function|def|class|return|if|elif|else|for|while|switch|case|break|continue|try|catch|finally|import|from|package|var|let|const|type|struct|interface|enum|impl|fn|pub|mod|use|select|insert|update|delete|create|drop|alter|begin|end|async|await|lambda)\b`)
	listLikeLinePattern        = regexp.MustCompile(`^(\d+[\.\)]|[A-Za-z][\.\)]|[IVXLCDMivxlcdm]+[\.\)]|[（(]?\d+[）)]|[一二三四五六七八九十百千]+[、\.\s]|[-*•])`)
	spacedAlphaSeqPattern      = regexp.MustCompile(`(?:\b[A-Za-z]\b(?: \b[A-Za-z]\b){1,})`)
	spaceBeforePunctPattern    = regexp.MustCompile(`\s+([,\)\]\};:])`)
	spaceAfterLeftPunctPattern = regexp.MustCompile(`([(\[{])\s+`)
	wordBeforeParenPattern     = regexp.MustCompile(`([A-Za-z0-9_])\s+\(`)
	multiSpacePattern          = regexp.MustCompile(`\s{2,}`)
)

var (
	pdfiumPool     pdfium.Pool
	pdfiumPoolOnce sync.Once
	pdfiumPoolErr  error
)

const (
	pdfiumInstanceTimeout    = 30 * time.Second
	formRowYTolerance        = 5.0
	structuredLineTolerance  = 3.0
	structuredWordGapMaximum = 4.0
)

func extractPDFMarkdownPages(ctx context.Context, data []byte) ([]string, error) {
	pdfiumPoolOnce.Do(func() {
		pdfiumPool, pdfiumPoolErr = webassembly.Init(webassembly.Config{
			MinIdle:  1,
			MaxIdle:  1,
			MaxTotal: 1,
		})
	})
	if pdfiumPoolErr != nil {
		return nil, fmt.Errorf("init pdfium webassembly pool failed: %w", pdfiumPoolErr)
	}

	instance, err := pdfiumPool.GetInstance(pdfiumInstanceTimeout)
	if err != nil {
		return nil, fmt.Errorf("get pdfium instance failed: %w", err)
	}
	defer instance.Close()

	doc, err := instance.OpenDocument(&requests.OpenDocument{
		File: &data,
	})
	if err != nil {
		return nil, fmt.Errorf("open pdf document failed: %w", err)
	}
	defer func() {
		_, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: doc.Document,
		})
	}()

	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return nil, fmt.Errorf("get pdf page count failed: %w", err)
	}

	markdownChunks := make([]string, 0, pageCountResp.PageCount)
	for pageIdx := 0; pageIdx < pageCountResp.PageCount; pageIdx++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		pageLines, err := extractStructuredPageLines(instance, doc, pageIdx)
		if err != nil {
			return nil, err
		}
		pageWords := flattenWordsFromLines(pageLines)
		if formMarkdown, ok := extractFormContentFromWords(pageWords); ok {
			formMarkdown = normalizePDFPlainText(formMarkdown)
			formMarkdown = escapeProbableCodeCommentHeadings(formMarkdown)
			if formMarkdown != "" && shouldUseFormMarkdown(formMarkdown) {
				markdownChunks = append(markdownChunks, formMarkdown)
				continue
			}
		}
		structuredMarkdown := normalizePDFPlainText(renderStructuredLinesMarkdown(pageLines))
		structuredMarkdown = escapeProbableCodeCommentHeadings(structuredMarkdown)
		if structuredMarkdown != "" && hasMarkdownHeading(structuredMarkdown) {
			markdownChunks = append(markdownChunks, structuredMarkdown)
			continue
		}

		plainText, err := extractPlainPageText(instance, doc, pageIdx)
		if err != nil {
			return nil, err
		}
		plainText = normalizePDFPlainText(plainText)
		plainText = escapeProbableCodeCommentHeadings(plainText)
		if plainText != "" {
			markdownChunks = append(markdownChunks, plainText)
		}
	}

	return markdownChunks, nil
}

func shouldUseFormMarkdown(content string) bool {
	lines := strings.Split(content, "\n")
	nonEmptyLines := 0
	tableLikeLines := 0
	totalNonTableTokens := 0
	singleRuneNonTableTokens := 0
	totalTableCells := 0
	nonEmptyTableCells := 0
	columnCountHist := make(map[int]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		nonEmptyLines++
		if strings.HasPrefix(trimmed, "|") {
			tableLikeLines++
			parts := strings.Split(trimmed, "|")
			if len(parts) >= 3 {
				cells := parts[1 : len(parts)-1]
				columnCountHist[len(cells)]++
				for _, cell := range cells {
					totalTableCells++
					if strings.TrimSpace(cell) != "" {
						nonEmptyTableCells++
					}
				}
			}
			continue
		}

		for _, token := range strings.Fields(trimmed) {
			totalNonTableTokens++
			if len([]rune(token)) == 1 {
				singleRuneNonTableTokens++
			}
		}
	}
	if nonEmptyLines == 0 || tableLikeLines < 3 {
		return false
	}
	if float64(tableLikeLines)/float64(nonEmptyLines) < 0.25 {
		return false
	}
	if totalTableCells > 0 {
		if float64(nonEmptyTableCells)/float64(totalTableCells) < 0.45 {
			return false
		}
	}
	if tableLikeLines > 0 {
		mostCommonColumnLines := 0
		for _, count := range columnCountHist {
			if count > mostCommonColumnLines {
				mostCommonColumnLines = count
			}
		}
		if float64(mostCommonColumnLines)/float64(tableLikeLines) < 0.60 {
			return false
		}
	}
	if totalNonTableTokens > 0 {
		if float64(singleRuneNonTableTokens)/float64(totalNonTableTokens) > 0.35 {
			return false
		}
	}
	return true
}

func extractStructuredPageLines(
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
	pageIdx int,
) ([]pdfTextLine, error) {
	structured, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIdx,
			},
		},
		Mode:                   requests.GetPageTextStructuredModeRects,
		CollectFontInformation: true,
	})
	if err != nil || len(structured.Rects) == 0 {
		return nil, nil
	}

	words := make([]pdfRect, 0, len(structured.Rects))
	for _, r := range structured.Rects {
		text := strings.TrimSpace(r.Text)
		if text == "" {
			continue
		}
		words = append(words, pdfRect{
			text:   text,
			left:   r.PointPosition.Left,
			top:    r.PointPosition.Top,
			right:  r.PointPosition.Right,
			bottom: r.PointPosition.Bottom,
		})
		if r.FontInformation != nil {
			words[len(words)-1].fontSize = r.FontInformation.Size
			words[len(words)-1].fontName = r.FontInformation.Name
		}
	}
	return buildStructuredLines(mergeStructuredCharsToWords(words)), nil
}

func flattenWordsFromLines(lines []pdfTextLine) []pdfRect {
	if len(lines) == 0 {
		return nil
	}
	words := make([]pdfRect, 0, len(lines)*8)
	for _, line := range lines {
		words = append(words, line.words...)
	}
	return words
}

func extractPlainPageText(
	instance pdfium.Pdfium,
	doc *responses.OpenDocument,
	pageIdx int,
) (string, error) {
	resp, err := instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIdx,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("get plain page text failed, page=%d: %w", pageIdx+1, err)
	}
	return resp.Text, nil
}

type pdfRect struct {
	text     string
	left     float64
	top      float64
	right    float64
	bottom   float64
	fontSize float64
	fontName string
}

type pdfTextLine struct {
	yKey     float64
	words    []pdfRect
	fontSize float64
	fontName string
}

type formRowInfo struct {
	yKey                float64
	words               []pdfRect
	text                string
	xGroups             []float64
	isParagraph         bool
	hasPartialNumbering bool
	isTableRow          bool
}

type rowRegion struct {
	start int
	end   int // exclusive
}

func extractFormContentFromWords(words []pdfRect) (string, bool) {
	if len(words) == 0 {
		return "", false
	}

	rowsByY := make(map[float64][]pdfRect)
	pageWidth := 0.0
	for _, word := range words {
		yKey := math.Round(word.top/formRowYTolerance) * formRowYTolerance
		rowsByY[yKey] = append(rowsByY[yKey], word)
		if word.right > pageWidth {
			pageWidth = word.right
		}
	}
	if len(rowsByY) == 0 {
		return "", false
	}
	if pageWidth <= 0 {
		pageWidth = 612
	}

	yKeys := make([]float64, 0, len(rowsByY))
	for y := range rowsByY {
		yKeys = append(yKeys, y)
	}
	sort.Slice(yKeys, func(i, j int) bool {
		return yKeys[i] > yKeys[j]
	})

	rowInfos := make([]formRowInfo, 0, len(yKeys))
	for _, yKey := range yKeys {
		rowWords := rowsByY[yKey]
		sort.Slice(rowWords, func(i, j int) bool {
			return rowWords[i].left < rowWords[j].left
		})
		if len(rowWords) == 0 {
			continue
		}

		firstX0 := rowWords[0].left
		lastX1 := rowWords[len(rowWords)-1].right
		lineWidth := lastX1 - firstX0
		combinedText := lineWordsToText(rowWords)

		xPositions := make([]float64, 0, len(rowWords))
		for _, word := range rowWords {
			xPositions = append(xPositions, word.left)
		}
		sort.Float64s(xPositions)

		xGroups := make([]float64, 0, len(xPositions))
		for _, x := range xPositions {
			if len(xGroups) == 0 || x-xGroups[len(xGroups)-1] > 50 {
				xGroups = append(xGroups, x)
			}
		}

		isParagraph := lineWidth > pageWidth*0.55 && len([]rune(combinedText)) > 60
		hasPartialNumbering := false
		if len(rowWords) > 0 && partialNumberingPattern.MatchString(strings.TrimSpace(rowWords[0].text)) {
			hasPartialNumbering = true
		}

		rowInfos = append(rowInfos, formRowInfo{
			yKey:                yKey,
			words:               rowWords,
			text:                combinedText,
			xGroups:             xGroups,
			isParagraph:         isParagraph,
			hasPartialNumbering: hasPartialNumbering,
		})
	}
	if len(rowInfos) == 0 {
		return "", false
	}

	allTableXPositions := make([]float64, 0)
	for _, info := range rowInfos {
		if len(info.xGroups) >= 3 && !info.isParagraph {
			allTableXPositions = append(allTableXPositions, info.xGroups...)
		}
	}
	if len(allTableXPositions) == 0 {
		return "", false
	}

	adaptiveTolerance := detectFormAdaptiveTolerance(allTableXPositions)
	globalColumns := buildGlobalColumns(allTableXPositions, adaptiveTolerance)
	if len(globalColumns) < 3 {
		return "", false
	}

	contentWidth := globalColumns[len(globalColumns)-1] - globalColumns[0]
	if contentWidth <= 0 {
		return "", false
	}

	avgColWidth := contentWidth / float64(len(globalColumns))
	if avgColWidth < 30 {
		return "", false
	}

	columnsPerInch := float64(len(globalColumns)) / (contentWidth / 72.0)
	if columnsPerInch > 10 {
		return "", false
	}

	adaptiveMaxColumns := int(20 * (pageWidth / 612))
	if adaptiveMaxColumns < 15 {
		adaptiveMaxColumns = 15
	}
	if len(globalColumns) > adaptiveMaxColumns {
		return "", false
	}

	for idx := range rowInfos {
		info := &rowInfos[idx]
		if info.isParagraph || info.hasPartialNumbering {
			info.isTableRow = false
			continue
		}

		alignedColumns := make(map[int]struct{})
		for _, word := range info.words {
			for colIdx, colX := range globalColumns {
				if math.Abs(word.left-colX) < 40 {
					alignedColumns[colIdx] = struct{}{}
					break
				}
			}
		}
		info.isTableRow = len(alignedColumns) >= 2
	}

	tableRegions := identifyRowTableRegions(rowInfos)
	totalTableRows := 0
	for _, region := range tableRegions {
		totalTableRows += region.end - region.start
	}
	if len(rowInfos) == 0 || float64(totalTableRows)/float64(len(rowInfos)) < 0.2 {
		return "", false
	}

	resultLines := make([]string, 0, len(rowInfos))
	for idx := 0; idx < len(rowInfos); {
		region := findRegionStartingAt(tableRegions, idx)
		if region != nil {
			tableData := make([][]string, 0, region.end-region.start)
			for rowIdx := region.start; rowIdx < region.end; rowIdx++ {
				tableData = append(tableData, extractCellsFromFormRow(rowInfos[rowIdx], globalColumns))
			}
			if isCodeLikeTableData(tableData) {
				codeBlock := renderCodeBlockFromTableData(tableData)
				if strings.TrimSpace(codeBlock) != "" {
					resultLines = append(resultLines, codeBlock)
				}
				idx = region.end
				continue
			}
			if !isLikelyTableData(tableData) {
				for rowIdx := region.start; rowIdx < region.end; rowIdx++ {
					resultLines = append(resultLines, rowInfos[rowIdx].text)
				}
				idx = region.end
				continue
			}
			tableMarkdown := toMarkdownTable(tableData, true)
			if strings.TrimSpace(tableMarkdown) != "" {
				resultLines = append(resultLines, tableMarkdown)
			}
			idx = region.end
			continue
		}

		if !isInsideTableRegion(tableRegions, idx) {
			resultLines = append(resultLines, rowInfos[idx].text)
		}
		idx++
	}

	return strings.Join(resultLines, "\n"), true
}

func lineWordsToText(words []pdfRect) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		trimmed := strings.TrimSpace(word.text)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " ")
}

func mergeStructuredCharsToWords(rects []pdfRect) []pdfRect {
	if len(rects) == 0 {
		return nil
	}

	rowsByY := make(map[float64][]pdfRect)
	for _, rect := range rects {
		yKey := math.Round(rect.top/structuredLineTolerance) * structuredLineTolerance
		rowsByY[yKey] = append(rowsByY[yKey], rect)
	}
	if len(rowsByY) == 0 {
		return nil
	}

	yKeys := make([]float64, 0, len(rowsByY))
	for y := range rowsByY {
		yKeys = append(yKeys, y)
	}
	sort.Slice(yKeys, func(i, j int) bool {
		return yKeys[i] > yKeys[j]
	})

	merged := make([]pdfRect, 0, len(rects))
	for _, yKey := range yKeys {
		rowRects := rowsByY[yKey]
		sort.Slice(rowRects, func(i, j int) bool {
			return rowRects[i].left < rowRects[j].left
		})
		if len(rowRects) == 0 {
			continue
		}

		current := rowRects[0]
		for idx := 1; idx < len(rowRects); idx++ {
			next := rowRects[idx]
			gap := next.left - current.right
			if gap <= structuredWordGapMaximum {
				current.text += next.text
				if next.right > current.right {
					current.right = next.right
				}
				if next.top > current.top {
					current.top = next.top
				}
				if next.bottom < current.bottom {
					current.bottom = next.bottom
				}
				if next.fontSize > current.fontSize {
					current.fontSize = next.fontSize
					current.fontName = next.fontName
				}
				continue
			}

			current.text = strings.TrimSpace(current.text)
			if current.text != "" {
				merged = append(merged, current)
			}
			current = next
		}

		current.text = strings.TrimSpace(current.text)
		if current.text != "" {
			merged = append(merged, current)
		}
	}

	return merged
}

func buildStructuredLines(words []pdfRect) []pdfTextLine {
	if len(words) == 0 {
		return nil
	}

	rowsByY := make(map[float64][]pdfRect)
	for _, word := range words {
		yKey := math.Round(word.top/formRowYTolerance) * formRowYTolerance
		rowsByY[yKey] = append(rowsByY[yKey], word)
	}
	if len(rowsByY) == 0 {
		return nil
	}

	yKeys := make([]float64, 0, len(rowsByY))
	for y := range rowsByY {
		yKeys = append(yKeys, y)
	}
	sort.Slice(yKeys, func(i, j int) bool {
		return yKeys[i] > yKeys[j]
	})

	lines := make([]pdfTextLine, 0, len(yKeys))
	for _, yKey := range yKeys {
		lineWords := rowsByY[yKey]
		sort.Slice(lineWords, func(i, j int) bool {
			return lineWords[i].left < lineWords[j].left
		})
		if len(lineWords) == 0 {
			continue
		}
		fontSize, fontName := dominantLineFont(lineWords)
		lines = append(lines, pdfTextLine{
			yKey:     yKey,
			words:    lineWords,
			fontSize: fontSize,
			fontName: fontName,
		})
	}
	return lines
}

func dominantLineFont(words []pdfRect) (float64, string) {
	type fontKey struct {
		size float64
		name string
	}
	weighted := make(map[fontKey]int)
	for _, word := range words {
		if word.fontSize <= 0 {
			continue
		}
		key := fontKey{
			size: math.Round(word.fontSize*10) / 10,
			name: word.fontName,
		}
		weighted[key] += len([]rune(strings.TrimSpace(word.text)))
	}
	if len(weighted) == 0 {
		return 0, ""
	}

	best := fontKey{}
	bestWeight := -1
	for key, weight := range weighted {
		if weight > bestWeight {
			best = key
			bestWeight = weight
		}
	}
	return best.size, best.name
}

func detectBodyFontSize(lines []pdfTextLine) float64 {
	weighted := make(map[float64]int)
	for _, line := range lines {
		if line.fontSize <= 0 {
			continue
		}
		size := math.Round(line.fontSize*10) / 10
		weighted[size] += len([]rune(lineWordsToText(line.words)))
	}
	if len(weighted) == 0 {
		return 0
	}

	bodySize := 0.0
	bestWeight := -1
	for size, weight := range weighted {
		if weight > bestWeight {
			bodySize = size
			bestWeight = weight
		}
	}
	return bodySize
}

func renderStructuredLinesMarkdown(lines []pdfTextLine) string {
	if len(lines) == 0 {
		return ""
	}

	bodyFontSize := detectBodyFontSize(lines)
	output := make([]string, 0, len(lines)*2)
	for idx, line := range lines {
		text := strings.TrimSpace(lineWordsToText(line.words))
		if text == "" {
			continue
		}
		level := detectHeadingLevel(line, bodyFontSize, text)
		if level > 0 {
			prevText := ""
			nextText := ""
			if idx > 0 {
				prevText = strings.TrimSpace(lineWordsToText(lines[idx-1].words))
			}
			if idx+1 < len(lines) {
				nextText = strings.TrimSpace(lineWordsToText(lines[idx+1].words))
			}
			if strings.HasPrefix(prevText, "$") || strings.HasPrefix(nextText, "$") {
				level = 0
			}
		}
		if level > 0 {
			if len(output) > 0 && output[len(output)-1] != "" {
				output = append(output, "")
			}
			output = append(output, strings.Repeat("#", level)+" "+text)
			output = append(output, "")
			continue
		}
		output = append(output, text)
	}

	return strings.TrimSpace(strings.Join(output, "\n"))
}

func detectHeadingLevel(line pdfTextLine, bodyFontSize float64, text string) int {
	if bodyFontSize <= 0 || line.fontSize <= 0 {
		return 0
	}
	textLen := len([]rune(text))
	if textLen < 2 || textLen > 40 {
		return 0
	}
	if strings.ContainsAny(text, "{}[]:=;") {
		return 0
	}
	if listLikeLinePattern.MatchString(strings.TrimSpace(text)) {
		return 0
	}
	if strings.HasSuffix(text, "。") || strings.HasSuffix(text, "，") || strings.HasSuffix(text, "：") {
		return 0
	}

	ratio := line.fontSize / bodyFontSize
	bold := isBoldFont(line.fontName)
	switch {
	case ratio >= 1.75 && bold:
		return 1
	case ratio >= 1.45 && bold:
		return 2
	case ratio >= 1.20 && bold:
		return 3
	case ratio >= 1.0 && bold && textLen <= 24:
		return 3
	default:
		return 0
	}
}

func isBoldFont(fontName string) bool {
	lower := strings.ToLower(fontName)
	return strings.Contains(lower, "bold") ||
		strings.Contains(lower, "black") ||
		strings.Contains(lower, "heavy") ||
		strings.HasSuffix(lower, "-bd") ||
		strings.HasSuffix(lower, "bd")
}

func hasMarkdownHeading(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") ||
			strings.HasPrefix(trimmed, "## ") ||
			strings.HasPrefix(trimmed, "### ") {
			return true
		}
	}
	return false
}

func escapeProbableCodeCommentHeadings(content string) string {
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		// Keep explicit markdown headings produced by the renderer.
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			continue
		}

		prev := nearestNonEmptyLine(lines, idx, -1)
		next := nearestNonEmptyLine(lines, idx, 1)
		if !looksLikeCodeContext(prev) && !looksLikeCodeContext(next) {
			continue
		}

		hashPos := strings.Index(line, "#")
		if hashPos < 0 {
			continue
		}
		if hashPos > 0 && line[hashPos-1] == '\\' {
			continue
		}
		lines[idx] = line[:hashPos] + `\` + line[hashPos:]
	}
	return strings.Join(lines, "\n")
}

func nearestNonEmptyLine(lines []string, start int, direction int) string {
	for idx := start + direction; idx >= 0 && idx < len(lines); idx += direction {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func looksLikeCodeContext(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "```") {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.ContainsAny(trimmed, "{}[]();:=<>") {
		return true
	}
	return false
}

func detectFormAdaptiveTolerance(positions []float64) float64 {
	if len(positions) < 4 {
		return 35
	}

	sortedPositions := append([]float64(nil), positions...)
	sort.Float64s(sortedPositions)

	gaps := make([]float64, 0, len(sortedPositions)-1)
	for idx := 0; idx < len(sortedPositions)-1; idx++ {
		gap := sortedPositions[idx+1] - sortedPositions[idx]
		if gap > 5 {
			gaps = append(gaps, gap)
		}
	}
	if len(gaps) < 3 {
		return 35
	}

	sort.Float64s(gaps)
	p70Idx := int(float64(len(gaps)) * 0.70)
	if p70Idx < 0 {
		p70Idx = 0
	}
	if p70Idx >= len(gaps) {
		p70Idx = len(gaps) - 1
	}

	tolerance := gaps[p70Idx]
	if tolerance < 25 {
		return 25
	}
	if tolerance > 50 {
		return 50
	}
	return tolerance
}

func buildGlobalColumns(positions []float64, tolerance float64) []float64 {
	if len(positions) == 0 {
		return nil
	}

	sortedPositions := append([]float64(nil), positions...)
	sort.Float64s(sortedPositions)

	columns := make([]float64, 0, len(sortedPositions))
	for _, x := range sortedPositions {
		if len(columns) == 0 || x-columns[len(columns)-1] > tolerance {
			columns = append(columns, x)
		}
	}
	return columns
}

func identifyRowTableRegions(rows []formRowInfo) []rowRegion {
	regions := make([]rowRegion, 0)
	for idx := 0; idx < len(rows); {
		if !rows[idx].isTableRow {
			idx++
			continue
		}
		start := idx
		for idx < len(rows) && rows[idx].isTableRow {
			idx++
		}
		if idx-start < 3 {
			continue
		}
		regions = append(regions, rowRegion{
			start: start,
			end:   idx,
		})
	}
	return regions
}

func findRegionStartingAt(regions []rowRegion, idx int) *rowRegion {
	for regionIdx := range regions {
		if regions[regionIdx].start == idx {
			return &regions[regionIdx]
		}
	}
	return nil
}

func isInsideTableRegion(regions []rowRegion, idx int) bool {
	for _, region := range regions {
		if idx > region.start && idx < region.end {
			return true
		}
	}
	return false
}

func extractCellsFromFormRow(row formRowInfo, globalColumns []float64) []string {
	numCols := len(globalColumns)
	cells := make([]string, numCols)
	for _, word := range row.words {
		assignedCol := numCols - 1
		for colIdx := 0; colIdx < numCols-1; colIdx++ {
			colEnd := globalColumns[colIdx+1]
			if word.left < colEnd-20 {
				assignedCol = colIdx
				break
			}
		}
		if cells[assignedCol] == "" {
			cells[assignedCol] = word.text
		} else {
			cells[assignedCol] += " " + word.text
		}
	}
	for idx := range cells {
		cells[idx] = strings.TrimSpace(cells[idx])
	}
	return cells
}

func toMarkdownTable(table [][]string, includeSeparator bool) string {
	if len(table) == 0 {
		return ""
	}

	normalized := make([][]string, 0, len(table))
	maxCols := 0
	for _, row := range table {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return ""
	}

	for _, row := range table {
		padded := make([]string, maxCols)
		for idx := 0; idx < maxCols; idx++ {
			if idx < len(row) {
				padded[idx] = strings.TrimSpace(row[idx])
			}
		}
		hasAny := false
		for _, cell := range padded {
			if strings.TrimSpace(cell) != "" {
				hasAny = true
				break
			}
		}
		if hasAny {
			normalized = append(normalized, padded)
		}
	}
	if len(normalized) == 0 {
		return ""
	}

	colWidths := make([]int, maxCols)
	for colIdx := 0; colIdx < maxCols; colIdx++ {
		maxWidth := 0
		for _, row := range normalized {
			width := len([]rune(row[colIdx]))
			if width > maxWidth {
				maxWidth = width
			}
		}
		colWidths[colIdx] = maxWidth
	}

	fmtRow := func(row []string) string {
		cells := make([]string, maxCols)
		for colIdx, width := range colWidths {
			cells[colIdx] = padRightRunes(row[colIdx], width)
		}
		return "|" + strings.Join(cells, "|") + "|"
	}

	if includeSeparator {
		header := normalized[0]
		rows := normalized[1:]

		lines := make([]string, 0, len(normalized)+1)
		lines = append(lines, fmtRow(header))

		divider := make([]string, 0, maxCols)
		for _, width := range colWidths {
			divider = append(divider, strings.Repeat("-", width))
		}
		lines = append(lines, "|"+strings.Join(divider, "|")+"|")

		for _, row := range rows {
			lines = append(lines, fmtRow(row))
		}
		return strings.Join(lines, "\n")
	}

	lines := make([]string, 0, len(normalized))
	for _, row := range normalized {
		lines = append(lines, fmtRow(row))
	}
	return strings.Join(lines, "\n")
}

func isLikelyTableData(table [][]string) bool {
	if len(table) < 3 {
		return false
	}

	totalNonEmptyCells := 0
	longCellCount := 0
	multiColumnRows := 0
	maxColumns := 0
	for _, row := range table {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}
	if maxColumns < 3 {
		return false
	}
	columnFillCounts := make([]int, maxColumns)
	for _, row := range table {
		rowNonEmpty := 0
		for colIdx, cell := range row {
			trimmed := strings.TrimSpace(cell)
			if trimmed == "" {
				continue
			}
			rowNonEmpty++
			totalNonEmptyCells++
			columnFillCounts[colIdx]++
			if len([]rune(trimmed)) > 30 {
				longCellCount++
			}
		}
		if rowNonEmpty >= 2 {
			multiColumnRows++
		}
	}

	if totalNonEmptyCells == 0 {
		return false
	}
	if float64(longCellCount)/float64(totalNonEmptyCells) > 0.30 {
		return false
	}
	if float64(multiColumnRows)/float64(len(table)) < 0.60 {
		return false
	}
	stableColumns := 0
	for _, fill := range columnFillCounts {
		if float64(fill)/float64(len(table)) >= 0.40 {
			stableColumns++
		}
	}
	if stableColumns < 3 {
		return false
	}
	return true
}

func isCodeLikeTableData(table [][]string) bool {
	if len(table) < 2 {
		return false
	}

	codeLikeRows := 0
	nonEmptyRows := 0
	for _, row := range table {
		line := normalizeCodeLine(buildLineFromCells(row))
		if line == "" {
			continue
		}
		nonEmptyRows++
		keywordHits := len(codeKeywordPattern.FindAllString(line, -1))
		symbolLike := strings.Contains(line, "//") || strings.ContainsAny(line, "{}()[]:=;")
		hasCodePunctuation := strings.ContainsAny(line, "().,:;=<>")
		if symbolLike || keywordHits >= 2 || (keywordHits >= 1 && hasCodePunctuation) {
			codeLikeRows++
		}
	}
	if nonEmptyRows == 0 {
		return false
	}
	if codeLikeRows < 2 {
		return false
	}
	return float64(codeLikeRows)/float64(nonEmptyRows) >= 0.35
}

func renderCodeBlockFromTableData(table [][]string) string {
	lines := make([]string, 0, len(table))
	for _, row := range table {
		line := normalizeCodeLine(buildLineFromCells(row))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "```\n" + strings.Join(lines, "\n") + "\n```"
}

func buildLineFromCells(row []string) string {
	parts := make([]string, 0, len(row))
	for _, cell := range row {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " ")
}

func normalizeCodeLine(line string) string {
	if line == "" {
		return ""
	}
	line = spacedAlphaSeqPattern.ReplaceAllStringFunc(line, func(segment string) string {
		return strings.ReplaceAll(segment, " ", "")
	})
	line = spaceBeforePunctPattern.ReplaceAllString(line, `$1`)
	line = spaceAfterLeftPunctPattern.ReplaceAllString(line, `$1`)
	line = wordBeforeParenPattern.ReplaceAllString(line, `$1(`)
	line = multiSpacePattern.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}

func padRightRunes(text string, width int) string {
	diff := width - len([]rune(text))
	if diff <= 0 {
		return text
	}
	return text + strings.Repeat(" ", diff)
}

func buildPDFMergedMarkdown(pageContents []string) string {
	if len(pageContents) == 0 {
		return ""
	}

	sections := make([]string, 0, len(pageContents))
	for _, pageContent := range pageContents {
		normalized := normalizePDFPlainText(pageContent)
		if normalized == "" {
			continue
		}
		sections = append(sections, normalized)
	}
	return strings.Join(sections, "\n\n")
}

func normalizePDFPlainText(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = mergePartialNumberingLines(content)

	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	pendingBlankLine := false
	for _, line := range lines {
		line = normalizeUnknownIcons(line)
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			pendingBlankLine = true
			continue
		}
		if pendingBlankLine && len(result) > 0 {
			result = append(result, "")
		}
		result = append(result, line)
		pendingBlankLine = false
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func normalizeUnknownIcons(text string) string {
	if text == "" {
		return ""
	}

	var builder strings.Builder
	prevWasUnknownIcon := false
	for _, r := range text {
		if r == '\uFFFD' || isPrivateUseRune(r) {
			if !prevWasUnknownIcon {
				builder.WriteRune('•')
			}
			prevWasUnknownIcon = true
			continue
		}
		prevWasUnknownIcon = false
		builder.WriteRune(r)
	}
	return builder.String()
}

func isPrivateUseRune(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) ||
		(r >= 0xF0000 && r <= 0xFFFFD) ||
		(r >= 0x100000 && r <= 0x10FFFD)
}

func mergePartialNumberingLines(content string) string {
	lines := strings.Split(content, "\n")
	merged := make([]string, 0, len(lines))

	for idx := 0; idx < len(lines); {
		current := strings.TrimSpace(lines[idx])
		if partialNumberingPattern.MatchString(current) {
			next := idx + 1
			for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
				next++
			}
			if next < len(lines) {
				merged = append(merged, current+" "+strings.TrimSpace(lines[next]))
				idx = next + 1
				continue
			}
		}

		merged = append(merged, lines[idx])
		idx++
	}

	return strings.Join(merged, "\n")
}

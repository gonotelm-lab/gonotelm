package xlsx

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Minimal decode of xl/charts/chartN.xml (drawingML chart).
type chartSpace struct {
	XMLName xml.Name  `xml:"chartSpace"`
	Chart   chartNode `xml:"chart"`
}

type chartNode struct {
	Title    *chartTitle `xml:"title"`
	PlotArea plotArea    `xml:"plotArea"`
}

type chartTitle struct {
	Tx *chartTx `xml:"tx"`
}

type chartTx struct {
	Rich *struct {
		Ps []struct {
			Rs []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"rich"`
	StrRef *struct {
		F        string `xml:"f"`
		StrCache *struct {
			Pts []chartPt `xml:"pt"`
		} `xml:"strCache"`
	} `xml:"strRef"`
}

type plotArea struct {
	Inner []byte `xml:",innerxml"`
}

type chartSer struct {
	Tx   *chartTx  `xml:"tx"`
	Cat  *chartCat `xml:"cat"`
	Val  *chartVal `xml:"val"`
	XVal *chartCat `xml:"xVal"`
	YVal *chartVal `xml:"yVal"`
}

type chartCat struct {
	StrRef *chartStrRef `xml:"strRef"`
	NumRef *chartNumRef `xml:"numRef"`
}

type chartVal struct {
	NumRef *chartNumRef `xml:"numRef"`
}

type chartStrRef struct {
	F        string         `xml:"f"`
	StrCache *chartStrCache `xml:"strCache"`
}

type chartNumRef struct {
	F        string         `xml:"f"`
	NumCache *chartNumCache `xml:"numCache"`
}

type chartStrCache struct {
	Pts []chartPt `xml:"pt"`
}

type chartNumCache struct {
	Pts []chartPt `xml:"pt"`
}

type chartPt struct {
	Idx int     `xml:"idx,attr"`
	V   *string `xml:"v"`
}

type extractedChart struct {
	Title     string
	Series    []extractedSeries
	SheetHint string
}

type extractedSeries struct {
	Name       string
	Categories []string
	Values     []string
}

var serBlockRE = regexp.MustCompile(`(?s)<(?:\w+:)?ser\b.*?</(?:\w+:)?ser>`)

func writeChartsForSheet(b *strings.Builder, f *excelize.File, sheet string, includeCharts bool) {
	if !includeCharts {
		return
	}
	for _, c := range extractCharts(f) {
		if c.SheetHint != "" && sheetRefMatches(c.SheetHint, sheet) {
			writeOneChart(b, c)
		}
	}
}

func writeOrphanCharts(b *strings.Builder, f *excelize.File, includeCharts bool) {
	if !includeCharts {
		return
	}
	var orphans []extractedChart
	for _, c := range extractCharts(f) {
		if c.SheetHint == "" {
			orphans = append(orphans, c)
		}
	}
	if len(orphans) == 0 {
		return
	}
	b.WriteString("\n## Charts\n")
	for _, c := range orphans {
		writeOneChart(b, c)
	}
}

func writeOneChart(b *strings.Builder, c extractedChart) {
	title := c.Title
	if title == "" {
		title = "Untitled"
	}
	fmt.Fprintf(b, "\n### Chart: %s\n\n", escapeCell(title))
	for _, ser := range c.Series {
		if ser.Name != "" {
			fmt.Fprintf(b, "Series: %s\n\n", escapeCell(ser.Name))
		}
		n := len(ser.Values)
		if len(ser.Categories) > n {
			n = len(ser.Categories)
		}
		if n == 0 {
			continue
		}
		b.WriteString("| Category | Value |\n| --- | --- |\n")
		for i := 0; i < n; i++ {
			cat := ""
			if i < len(ser.Categories) {
				cat = ser.Categories[i]
			}
			val := ""
			if i < len(ser.Values) {
				val = ser.Values[i]
			}
			fmt.Fprintf(b, "| %s | %s |\n", escapeCell(cat), escapeCell(val))
		}
		b.WriteByte('\n')
	}
}

func extractCharts(f *excelize.File) []extractedChart {
	var out []extractedChart
	f.Pkg.Range(func(k, v any) bool {
		key, ok := k.(string)
		if !ok || !strings.Contains(key, "xl/charts/chart") || strings.Contains(key, "charts/_rels") {
			return true
		}
		raw, ok := v.([]byte)
		if !ok || len(raw) == 0 {
			return true
		}
		if c, ok := parseChartXML(f, raw); ok {
			out = append(out, c)
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i].Title < out[j].Title
	})
	return out
}

func parseChartXML(f *excelize.File, raw []byte) (extractedChart, bool) {
	normalized := []byte(stripPrefixesProper(xmlnsAttrRE.ReplaceAllString(string(raw), "")))
	var space chartSpace
	if err := xml.Unmarshal(normalized, &space); err != nil {
		return extractedChart{}, false
	}

	title := chartTitleText(space.Chart.Title)
	sers := findSeries(space.Chart.PlotArea.Inner)
	if title == "" && len(sers) == 0 {
		return extractedChart{}, false
	}

	ec := extractedChart{Title: title}
	for _, ser := range sers {
		es := extractedSeries{
			Name:       resolveSeriesName(f, ser.Tx),
			Categories: resolveCatValues(f, firstCat(ser)),
			Values:     resolveNumValues(f, firstVal(ser)),
		}
		if es.Name == "" && len(es.Categories) == 0 && len(es.Values) == 0 {
			continue
		}
		ec.Series = append(ec.Series, es)
		if ec.SheetHint == "" {
			ec.SheetHint = sheetHintFromSeries(ser)
		}
	}
	return ec, true
}

func findSeries(inner []byte) []chartSer {
	if len(inner) == 0 {
		return nil
	}
	normalizedInner := []byte(stripPrefixesProper(xmlnsAttrRE.ReplaceAllString(string(inner), "")))
	blocks := serBlockRE.FindAll(normalizedInner, -1)
	out := make([]chartSer, 0, len(blocks))
	for _, block := range blocks {
		var ser chartSer
		if err := xml.Unmarshal(block, &ser); err != nil {
			continue
		}
		out = append(out, ser)
	}
	return out
}

var xmlnsAttrRE = regexp.MustCompile(`\sxmlns(:\w+)?="[^"]*"`)

func stripPrefixesProper(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '<' {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte('<')
		i++
		if i >= len(s) {
			break
		}
		// passthrough comments / processing instructions / DOCTYPE
		if s[i] == '!' || s[i] == '?' {
			for i < len(s) && s[i] != '>' {
				b.WriteByte(s[i])
				i++
			}
			continue
		}
		if s[i] == '/' {
			b.WriteByte('/')
			i++
		}
		start := i
		for i < len(s) && s[i] != '>' && s[i] != ' ' && s[i] != '/' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			i++
		}
		name := s[start:i]
		if idx := strings.IndexByte(name, ':'); idx >= 0 {
			name = name[idx+1:]
		}
		b.WriteString(name)
		// copy attributes and closing until '>'
		for i < len(s) {
			b.WriteByte(s[i])
			if s[i] == '>' {
				i++
				break
			}
			i++
		}
	}
	return b.String()
}

func chartTitleText(t *chartTitle) string {
	if t == nil || t.Tx == nil {
		return ""
	}
	if t.Tx.Rich != nil {
		var parts []string
		for _, p := range t.Tx.Rich.Ps {
			for _, r := range p.Rs {
				if r.T != "" {
					parts = append(parts, r.T)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	}
	if t.Tx.StrRef != nil {
		if t.Tx.StrRef.StrCache != nil {
			vals := ptsToStrings(t.Tx.StrRef.StrCache.Pts)
			if len(vals) > 0 {
				return vals[0]
			}
		}
		if lit := strings.TrimSpace(t.Tx.StrRef.F); lit != "" && !strings.Contains(lit, "!") {
			return lit
		}
	}
	return ""
}

func firstCat(ser chartSer) *chartCat {
	if ser.Cat != nil {
		return ser.Cat
	}
	return ser.XVal
}

func firstVal(ser chartSer) *chartVal {
	if ser.Val != nil {
		return ser.Val
	}
	return ser.YVal
}

func resolveSeriesName(f *excelize.File, tx *chartTx) string {
	if tx == nil {
		return ""
	}
	if tx.Rich != nil {
		var parts []string
		for _, p := range tx.Rich.Ps {
			for _, r := range p.Rs {
				if r.T != "" {
					parts = append(parts, r.T)
				}
			}
		}
		if s := strings.TrimSpace(strings.Join(parts, "")); s != "" {
			return s
		}
	}
	if tx.StrRef != nil {
		if tx.StrRef.StrCache != nil {
			vals := ptsToStrings(tx.StrRef.StrCache.Pts)
			if len(vals) > 0 {
				return vals[0]
			}
		}
		fml := strings.TrimSpace(tx.StrRef.F)
		if fml == "" {
			return ""
		}
		if !strings.Contains(fml, "!") {
			return fml
		}
		vals := readFormulaValues(f, fml)
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func resolveCatValues(f *excelize.File, cat *chartCat) []string {
	if cat == nil {
		return nil
	}
	if cat.StrRef != nil {
		if cat.StrRef.StrCache != nil {
			if vals := ptsToStrings(cat.StrRef.StrCache.Pts); len(vals) > 0 {
				return vals
			}
		}
		return readFormulaValues(f, cat.StrRef.F)
	}
	if cat.NumRef != nil {
		if cat.NumRef.NumCache != nil {
			if vals := ptsToStrings(cat.NumRef.NumCache.Pts); len(vals) > 0 {
				return vals
			}
		}
		return readFormulaValues(f, cat.NumRef.F)
	}
	return nil
}

func resolveNumValues(f *excelize.File, val *chartVal) []string {
	if val == nil || val.NumRef == nil {
		return nil
	}
	if val.NumRef.NumCache != nil {
		if vals := ptsToStrings(val.NumRef.NumCache.Pts); len(vals) > 0 {
			return vals
		}
	}
	return readFormulaValues(f, val.NumRef.F)
}

func ptsToStrings(pts []chartPt) []string {
	if len(pts) == 0 {
		return nil
	}
	sorted := append([]chartPt(nil), pts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Idx < sorted[j].Idx })
	out := make([]string, 0, len(sorted))
	for _, p := range sorted {
		if p.V != nil {
			out = append(out, *p.V)
		} else {
			out = append(out, "")
		}
	}
	return out
}

func sheetHintFromSeries(ser chartSer) string {
	candidates := []string{}
	if ser.Cat != nil && ser.Cat.StrRef != nil {
		candidates = append(candidates, ser.Cat.StrRef.F)
	}
	if ser.Cat != nil && ser.Cat.NumRef != nil {
		candidates = append(candidates, ser.Cat.NumRef.F)
	}
	if ser.Val != nil && ser.Val.NumRef != nil {
		candidates = append(candidates, ser.Val.NumRef.F)
	}
	if ser.XVal != nil && ser.XVal.StrRef != nil {
		candidates = append(candidates, ser.XVal.StrRef.F)
	}
	if ser.YVal != nil && ser.YVal.NumRef != nil {
		candidates = append(candidates, ser.YVal.NumRef.F)
	}
	for _, c := range candidates {
		if sheet, _, _, ok := parseSheetRangeRef(c); ok {
			return sheet
		}
	}
	return ""
}

func sheetRefMatches(hint, sheet string) bool {
	return strings.EqualFold(strings.TrimSpace(hint), strings.TrimSpace(sheet))
}

var sheetRangeRE = regexp.MustCompile(`^(?:'?([^']+)'?|([^!]+))!(.+)$`)

func parseSheetRangeRef(ref string) (sheet, start, end string, ok bool) {
	ref = strings.TrimSpace(ref)
	ref = strings.ReplaceAll(ref, "$", "")
	m := sheetRangeRE.FindStringSubmatch(ref)
	if m == nil {
		return "", "", "", false
	}
	sheet = m[1]
	if sheet == "" {
		sheet = m[2]
	}
	rng := m[3]
	parts := strings.Split(rng, ":")
	start = parts[0]
	end = parts[0]
	if len(parts) > 1 {
		end = parts[1]
	}
	return sheet, start, end, true
}

func readFormulaValues(f *excelize.File, formula string) []string {
	sheet, start, end, ok := parseSheetRangeRef(formula)
	if !ok {
		return nil
	}
	c1, r1, err1 := excelize.CellNameToCoordinates(start)
	c2, r2, err2 := excelize.CellNameToCoordinates(end)
	if err1 != nil || err2 != nil {
		return nil
	}
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	out := make([]string, 0, (c2-c1+1)*(r2-r1+1))
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			cell, err := excelize.CoordinatesToCellName(c, r)
			if err != nil {
				continue
			}
			v, err := f.GetCellValue(sheet, cell)
			if err != nil {
				out = append(out, "")
				continue
			}
			out = append(out, v)
		}
	}
	return out
}

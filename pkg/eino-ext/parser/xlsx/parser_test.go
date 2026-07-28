package xlsx

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func buildWorkbook(t *testing.T, build func(*excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	t.Cleanup(func() { _ = f.Close() })
	build(f)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	return buf.Bytes()
}

func TestXLSXParser_MultiSheetAndEmpty(t *testing.T) {
	data := buildWorkbook(t, func(f *excelize.File) {
		_ = f.SetCellValue("Sheet1", "A1", "a")
		_ = f.SetCellValue("Sheet1", "B1", "1")
		_ = f.SetCellValue("Sheet1", "A2", "b")
		_ = f.SetCellValue("Sheet1", "B2", "2")
		_, _ = f.NewSheet("EmptySheet")
	})

	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs)=%d want 1", len(docs))
	}
	content := docs[0].Content
	if !strings.Contains(content, "## Sheet1") {
		t.Fatalf("missing Sheet1 heading:\n%s", content)
	}
	if !strings.Contains(content, "| Col1 | Col2 |") {
		t.Fatalf("missing synthetic header:\n%s", content)
	}
	if !strings.Contains(content, "| a | 1 |") || !strings.Contains(content, "| b | 2 |") {
		t.Fatalf("missing data rows:\n%s", content)
	}
	if !strings.Contains(content, "## EmptySheet") || !strings.Contains(content, "(empty)") {
		t.Fatalf("empty sheet not handled:\n%s", content)
	}
	if idx1, idx2 := strings.Index(content, "## Sheet1"), strings.Index(content, "## EmptySheet"); idx1 < 0 || idx2 < idx1 {
		t.Fatalf("sheet order wrong:\n%s", content)
	}
}

func TestXLSXParser_EscapeAndComments(t *testing.T) {
	data := buildWorkbook(t, func(f *excelize.File) {
		_ = f.SetCellValue("Sheet1", "A1", "a|b")
		_ = f.SetCellValue("Sheet1", "B1", "line1\nline2")
		_ = f.AddComment("Sheet1", excelize.Comment{
			Cell: "A1",
			Text: "hello note",
		})
	})

	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := docs[0].Content
	if !strings.Contains(content, `a\|b`) {
		t.Fatalf("pipe not escaped:\n%s", content)
	}
	if !strings.Contains(content, "line1 line2") {
		t.Fatalf("newline not flattened:\n%s", content)
	}
	if !strings.Contains(content, "### Comments") {
		t.Fatalf("missing Comments section:\n%s", content)
	}
	if !strings.Contains(content, "`A1`:") || !strings.Contains(content, "hello note") {
		t.Fatalf("comment body missing:\n%s", content)
	}
}

func TestXLSXParser_NoCommentsSectionWhenEmpty(t *testing.T) {
	data := buildWorkbook(t, func(f *excelize.File) {
		_ = f.SetCellValue("Sheet1", "A1", "x")
	})
	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strings.Contains(docs[0].Content, "### Comments") {
		t.Fatalf("unexpected Comments:\n%s", docs[0].Content)
	}
}

func TestXLSXParser_ChartFromMetadata(t *testing.T) {
	data := buildWorkbook(t, func(f *excelize.File) {
		for i, v := range []any{"HP", "DELL", "Lenovo"} {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
		for i, v := range []any{200, 450, 200} {
			cell, _ := excelize.CoordinatesToCellName(2, i+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
		err := f.AddChart("Sheet1", "E1", &excelize.Chart{
			Type: excelize.Col,
			Series: []excelize.ChartSeries{{
				Name:       "Brand",
				Categories: "Sheet1!$A$1:$A$3",
				Values:     "Sheet1!$B$1:$B$3",
			}},
			Title: excelize.ChartTitle{
				Paragraph: []excelize.RichTextRun{{Text: "Brand"}},
			},
		})
		if err != nil {
			t.Fatalf("AddChart: %v", err)
		}
	})

	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := docs[0].Content
	if !strings.Contains(content, "### Chart: Brand") {
		t.Fatalf("missing chart title:\n%s", content)
	}
	if !strings.Contains(content, "| HP | 200 |") {
		t.Fatalf("missing chart row HP:\n%s", content)
	}
	if !strings.Contains(content, "| DELL | 450 |") {
		t.Fatalf("missing chart row DELL:\n%s", content)
	}
}

func TestXLSXParser_CommonChartTypesExtractData(t *testing.T) {
	types := []struct {
		name      string
		chartType excelize.ChartType
		anchor    string
	}{
		{"Line", excelize.Line, "E1"},
		{"Area", excelize.Area, "E18"},
		{"Bar", excelize.Bar, "E35"},
		{"Pie", excelize.Pie, "E52"},
		{"Doughnut", excelize.Doughnut, "L1"},
		{"Radar", excelize.Radar, "L18"},
		{"Scatter", excelize.Scatter, "L35"},
	}

	data := buildWorkbook(t, func(f *excelize.File) {
		cats := []any{"A", "B", "C"}
		vals := []any{10, 20, 30}
		for i, v := range cats {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(2, i+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
		// Scatter typically uses numeric X; put numbers in col C as well.
		for i, v := range []any{1, 2, 3} {
			cell, _ := excelize.CoordinatesToCellName(3, i+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}

		for _, tc := range types {
			categories := "Sheet1!$A$1:$A$3"
			values := "Sheet1!$B$1:$B$3"
			if tc.chartType == excelize.Scatter {
				categories = "Sheet1!$C$1:$C$3"
			}
			err := f.AddChart("Sheet1", tc.anchor, &excelize.Chart{
				Type: tc.chartType,
				Series: []excelize.ChartSeries{{
					Name:       tc.name,
					Categories: categories,
					Values:     values,
				}},
				Title: excelize.ChartTitle{
					Paragraph: []excelize.RichTextRun{{Text: tc.name}},
				},
			})
			if err != nil {
				t.Fatalf("AddChart %s: %v", tc.name, err)
			}
		}
	})

	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := docs[0].Content
	t.Logf("parsed charts markdown:\n%s", content)

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(content, "### Chart: "+tc.name) {
				t.Fatalf("missing chart title for %s:\n%s", tc.name, content)
			}
			if tc.chartType == excelize.Scatter {
				// X from col C, Y from col B
				if !strings.Contains(content, "| 1 | 10 |") ||
					!strings.Contains(content, "| 2 | 20 |") ||
					!strings.Contains(content, "| 3 | 30 |") {
					t.Fatalf("scatter data missing for %s:\n%s", tc.name, content)
				}
				return
			}
			if !strings.Contains(content, "| A | 10 |") ||
				!strings.Contains(content, "| B | 20 |") ||
				!strings.Contains(content, "| C | 30 |") {
				t.Fatalf("series data missing for %s:\n%s", tc.name, content)
			}
		})
	}
}

func TestXLSXParser_Parse(t *testing.T) {
	f, err := os.ReadFile("testdata/Book1.xlsx")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	docs, err := NewXLSXParser(nil).Parse(context.Background(), bytes.NewReader(f))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := docs[0].Content
	if !strings.Contains(content, "## Sheet1") {
		t.Fatalf("missing Sheet1 heading:\n%s", content)
	}

	t.Logf("content: %s", content)
}

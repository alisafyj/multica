package designcore

import (
	"math"
	"reflect"
	"testing"
)

func TestMeasureTextWidthUsesUnicodeCodePointFactors(t *testing.T) {
	metrics := TypographyMetrics{FontSize: 10, LetterSpacing: 1}

	tests := []struct {
		name string
		text string
		want float64
	}{
		{name: "empty", text: "", want: 0},
		{name: "ascii categories", text: "Ab3 !", want: 29.3},
		{name: "cjk", text: "中", want: 10},
		{name: "full width punctuation", text: "，", want: 10},
		{name: "combining mark", text: "e\u0301", want: 6.6},
		{name: "non cjk script", text: "م", want: 10},
		{name: "emoji", text: "😀", want: 10},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MeasureTextWidth(test.text, metrics); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("MeasureTextWidth(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestMeasureTextWidthAddsLetterSpacingBetweenAdjacentRunes(t *testing.T) {
	metrics := TypographyMetrics{FontSize: 10, LetterSpacing: 2}
	if got, want := MeasureTextWidth("ab", metrics), 13.2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("MeasureTextWidth = %v, want %v", got, want)
	}
}

func TestAllocateTableLayoutUsesPreferredWidthsWithinViewport(t *testing.T) {
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns: []TableColumnSpec{
			{Key: "customer", Title: "Customer", Cell: "text", Width: "medium"},
			{Key: "phone", Title: "Phone", Cell: "text", Width: "medium"},
		},
		ViewportWidth:         400,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if layout.HorizontalScroll || layout.TotalWidth != 360 {
		t.Fatalf("layout = %+v", layout)
	}
	if got := layout.Columns; got[0].Width != 180 || got[1].Width != 180 || got[1].X != 180 || got[0].Pinned != "left" {
		t.Fatalf("columns = %+v", got)
	}
}

func TestAllocateTableLayoutExpandsFlexibleColumnsAfterPreferredWidths(t *testing.T) {
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns: []TableColumnSpec{
			{Key: "name", Title: "Name", Cell: "text", Width: "medium"},
			{Key: "notes", Title: "Notes", Cell: "text", Width: "flexible"},
		},
		ViewportWidth:         600,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if layout.HorizontalScroll || layout.TotalWidth != 600 {
		t.Fatalf("layout = %+v", layout)
	}
	if got := layout.Columns; got[0].Width != 180 || got[1].Width != 420 || got[1].X != 180 {
		t.Fatalf("columns = %+v", got)
	}
}

func TestAllocateTableLayoutUsesMeasuredChineseAndASCIIContent(t *testing.T) {
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns: []TableColumnSpec{
			{Key: "chinese", Title: "名称", Cell: "text", Width: "wide"},
			{Key: "ascii", Title: "Code", Cell: "text", Width: "wide"},
		},
		Rows: []map[string]string{{
			"chinese": "一个用于验证宽度分配的很长示例客户名称",
			"ascii":   "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
		}},
		ViewportWidth:         1000,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if got := layout.Columns; got[0].PreferredWidth <= 260 || got[0].PreferredWidth > 420 || got[1].PreferredWidth != 420 {
		t.Fatalf("columns = %+v", got)
	}
}

func TestAllocateTableLayoutUsesHorizontalScrollBeforeOverlap(t *testing.T) {
	columns := []TableColumnSpec{
		{Key: "customerName", Title: "客户姓名", Cell: "text", Width: "wide"},
		{Key: "phone", Title: "手机号", Cell: "text", Width: "medium"},
		{Key: "company", Title: "所属公司", Cell: "text", Width: "wide"},
		{Key: "status", Title: "客户状态", Cell: "status-tag", Width: "narrow"},
	}
	rows := []map[string]string{{"customerName": "一个用于验证宽度分配的很长示例客户名称", "phone": "13800000001", "company": "示例科技有限公司华东业务中心", "status": "待跟进"}}
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{Columns: columns, Rows: rows, RowActionCount: 2, ViewportWidth: 620, Typography: TypographyMetrics{FontSize: 14}, CellHorizontalPadding: 16})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if !layout.HorizontalScroll || layout.TotalWidth <= 620 {
		t.Fatalf("layout = %+v", layout)
	}
	for _, column := range layout.Columns {
		if column.Width < column.MinWidth {
			t.Fatalf("column below minimum: %+v", column)
		}
	}
	if layout.ActionColumn == nil || layout.ActionColumn.Width != 176 || layout.ActionColumn.X != 636 || layout.ActionColumn.Pinned != "right" {
		t.Fatalf("action column = %+v", layout.ActionColumn)
	}
}

func TestAllocateTableLayoutCapsActionReserveAndOmitsZeroActions(t *testing.T) {
	input := TableLayoutInput{
		Columns:               []TableColumnSpec{{Key: "status", Title: "Status", Cell: "status-tag", Width: "narrow"}},
		ViewportWidth:         400,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	}

	layout, diagnostics := AllocateTableLayout(input)
	if diagnostics.HasErrors() || layout.ActionColumn != nil || layout.Columns[0].Width != 120 {
		t.Fatalf("zero actions layout = %+v, diagnostics = %+v", layout, diagnostics)
	}

	input.RowActionCount = 3
	layout, diagnostics = AllocateTableLayout(input)
	if diagnostics.HasErrors() || layout.ActionColumn == nil || layout.ActionColumn.Width != 176 || layout.ActionColumn.X != 120 || layout.TotalWidth != 296 {
		t.Fatalf("capped actions layout = %+v, diagnostics = %+v", layout, diagnostics)
	}
}

func TestAllocateTableLayoutMaintainsBoundsAndPositions(t *testing.T) {
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns: []TableColumnSpec{
			{Key: "a", Title: "A", Cell: "text", Width: "narrow"},
			{Key: "b", Title: "B", Cell: "text", Width: "flexible"},
		},
		RowActionCount:        1,
		ViewportWidth:         2000,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	lastX := 0.0
	for _, column := range layout.Columns {
		if column.Width < column.MinWidth || column.Width > column.MaxWidth {
			t.Fatalf("column bounds = %+v", column)
		}
		if column.X != lastX {
			t.Fatalf("column X = %v, want %v", column.X, lastX)
		}
		lastX += column.Width
	}
	if layout.ActionColumn == nil || layout.ActionColumn.X != lastX || layout.TotalWidth != lastX+layout.ActionColumn.Width || layout.HorizontalScroll {
		t.Fatalf("layout = %+v", layout)
	}
}

func TestAllocateTableLayoutRejectsInvalidInputWithTypedDiagnostics(t *testing.T) {
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns:               []TableColumnSpec{{Key: "bad", Title: "Bad", Cell: "text", Width: "unsupported"}},
		RowActionCount:        -1,
		ViewportWidth:         math.NaN(),
		Typography:            TypographyMetrics{FontSize: math.Inf(1), LetterSpacing: -1},
		CellHorizontalPadding: math.Inf(1),
	})
	if !diagnostics.HasErrors() || len(diagnostics) != 6 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != DiagnosticError {
			t.Fatalf("diagnostic = %+v", diagnostic)
		}
	}
	if len(layout.Columns) != 0 || layout.ActionColumn != nil || layout.TotalWidth != 0 || layout.HorizontalScroll {
		t.Fatalf("invalid layout = %+v", layout)
	}
}

func TestAllocateTableLayoutDoesNotMutateInput(t *testing.T) {
	input := TableLayoutInput{
		Columns:               []TableColumnSpec{{Key: "name", Title: "Name", Cell: "text", Width: "wide"}},
		Rows:                  []map[string]string{{"name": "Example"}},
		RowActionCount:        1,
		ViewportWidth:         500,
		Typography:            TypographyMetrics{FontSize: 14},
		CellHorizontalPadding: 16,
	}
	want := input
	want.Columns = append([]TableColumnSpec(nil), input.Columns...)
	want.Rows = []map[string]string{{"name": "Example"}}

	_, diagnostics := AllocateTableLayout(input)
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if !reflect.DeepEqual(input, want) {
		t.Fatalf("input mutated: got %+v, want %+v", input, want)
	}
}

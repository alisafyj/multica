package designcore

import (
	"math"
	"strconv"
)

type TableLayoutInput struct {
	Columns               []TableColumnSpec
	Rows                  []map[string]string
	RowActionCount        int
	ViewportWidth         float64
	Typography            TypographyMetrics
	CellHorizontalPadding float64
}

type TableColumnLayout struct {
	Key                      string
	X, Width, MinWidth       float64
	PreferredWidth, MaxWidth float64
	Pinned                   string
}

type TableLayout struct {
	Columns          []TableColumnLayout
	ActionColumn     *TableColumnLayout
	TotalWidth       float64
	HorizontalScroll bool
}

type tableWidthBounds struct {
	min       float64
	preferred float64
	max       float64
	flexible  bool
}

var tableWidthHints = map[string]tableWidthBounds{
	"narrow":   {min: 96, preferred: 120, max: 160},
	"medium":   {min: 140, preferred: 180, max: 260},
	"wide":     {min: 200, preferred: 260, max: 420},
	"flexible": {min: 160, preferred: 240, max: 600, flexible: true},
}

func AllocateTableLayout(input TableLayoutInput) (TableLayout, Diagnostics) {
	diagnostics := validateTableLayoutInput(input)
	if diagnostics.HasErrors() {
		return TableLayout{}, diagnostics
	}

	columns := make([]TableColumnLayout, len(input.Columns))
	minimumTotal := actionReserve(input.RowActionCount)
	for index, column := range input.Columns {
		bounds := tableWidthHints[column.Width]
		preferred := bounds.preferred
		contentWidth := MeasureTextWidth(column.Title, input.Typography)
		for _, row := range input.Rows {
			contentWidth = math.Max(contentWidth, MeasureTextWidth(row[column.Key], input.Typography))
		}
		preferred = math.Min(bounds.max, math.Max(preferred, addWidth(contentWidth, input.CellHorizontalPadding*2)))
		columns[index] = TableColumnLayout{
			Key:            column.Key,
			Width:          bounds.min,
			MinWidth:       bounds.min,
			PreferredWidth: preferred,
			MaxWidth:       bounds.max,
		}
		minimumTotal += bounds.min
	}

	layout := TableLayout{Columns: columns}
	if minimumTotal > input.ViewportWidth {
		layout.HorizontalScroll = true
	} else {
		remaining := input.ViewportWidth - minimumTotal
		remaining = distributeColumnGrowth(layout.Columns, remaining, func(column TableColumnLayout) float64 {
			return column.PreferredWidth
		}, func(int) bool {
			return true
		})
		remaining = distributeColumnGrowth(layout.Columns, remaining, func(column TableColumnLayout) float64 {
			return column.MaxWidth
		}, func(index int) bool {
			return tableWidthHints[input.Columns[index].Width].flexible
		})
	}

	x := 0.0
	for index := range layout.Columns {
		layout.Columns[index].X = x
		x += layout.Columns[index].Width
	}
	if reserve := actionReserve(input.RowActionCount); reserve > 0 {
		actionColumn := TableColumnLayout{
			Key:            "actions",
			X:              x,
			Width:          reserve,
			MinWidth:       reserve,
			PreferredWidth: reserve,
			MaxWidth:       reserve,
		}
		layout.ActionColumn = &actionColumn
		x += reserve
	}
	layout.TotalWidth = x
	return layout, diagnostics
}

func validateTableLayoutInput(input TableLayoutInput) Diagnostics {
	diagnostics := Diagnostics{}
	if input.RowActionCount < 0 {
		diagnostics = append(diagnostics, tableLayoutDiagnostic("invalid_row_action_count", "row action count must not be negative", "rowActionCount"))
	}
	if !isNonNegativeFinite(input.ViewportWidth) {
		diagnostics = append(diagnostics, tableLayoutDiagnostic("invalid_viewport_width", "viewport width must be finite and non-negative", "viewportWidth"))
	}
	if !isNonNegativeFinite(input.Typography.FontSize) {
		diagnostics = append(diagnostics, tableLayoutDiagnostic("invalid_font_size", "font size must be finite and non-negative", "typography.fontSize"))
	}
	if !isFinite(input.Typography.LetterSpacing) {
		diagnostics = append(diagnostics, tableLayoutDiagnostic("invalid_letter_spacing", "letter spacing must be finite", "typography.letterSpacing"))
	}
	if !isNonNegativeFinite(input.CellHorizontalPadding) {
		diagnostics = append(diagnostics, tableLayoutDiagnostic("invalid_cell_horizontal_padding", "cell horizontal padding must be finite and non-negative", "cellHorizontalPadding"))
	}
	for index, column := range input.Columns {
		if _, ok := tableWidthHints[column.Width]; !ok {
			diagnostics = append(diagnostics, tableLayoutDiagnostic("unsupported_table_column_width", "table column width must be narrow, medium, wide, or flexible", tableColumnPath(index, "width")))
		}
	}
	return diagnostics
}

func tableLayoutDiagnostic(code, message, path string) Diagnostic {
	return Diagnostic{Code: code, Severity: DiagnosticError, Message: message, Paths: []string{path}}
}

func tableColumnPath(index int, field string) string {
	return "columns[" + strconv.Itoa(index) + "]." + field
}

func actionReserve(actionCount int) float64 {
	if actionCount == 0 {
		return 0
	}
	if actionCount == 1 {
		return 88
	}
	return 176
}

func distributeColumnGrowth(columns []TableColumnLayout, remaining float64, target func(TableColumnLayout) float64, eligible func(int) bool) float64 {
	if remaining <= 0 {
		return remaining
	}

	indices := make([]int, 0, len(columns))
	totalDemand := 0.0
	for index, column := range columns {
		if !eligible(index) {
			continue
		}
		demand := target(column) - column.Width
		if demand <= 0 {
			continue
		}
		indices = append(indices, index)
		totalDemand += demand
	}
	if totalDemand <= 0 {
		return remaining
	}
	if totalDemand <= remaining {
		for _, index := range indices {
			columns[index].Width = target(columns[index])
		}
		return remaining - totalDemand
	}

	budget := remaining
	allocated := 0.0
	for position, index := range indices {
		column := &columns[index]
		demand := target(*column) - column.Width
		growth := budget * demand / totalDemand
		if position == len(indices)-1 {
			growth = budget - allocated
		}
		growth = math.Min(demand, math.Max(0, growth))
		column.Width += growth
		allocated += growth
	}

	residual := budget - allocated
	for _, index := range indices {
		if residual <= 0 {
			break
		}
		column := &columns[index]
		growth := math.Min(target(*column)-column.Width, residual)
		column.Width += growth
		allocated += growth
		residual -= growth
	}
	return remaining - allocated
}

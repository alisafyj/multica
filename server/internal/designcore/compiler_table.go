package designcore

import (
	"fmt"
	"math"
)

func (c *listPageCompiler) instantiateTable(tableY float64) (float64, error) {
	constraints := c.input.Blueprint.Constraints
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{
		Columns:               c.input.PageSpec.Table.Columns,
		Rows:                  c.input.PageSpec.Table.SampleRows,
		RowActionCount:        len(c.input.PageSpec.Table.RowActions),
		ViewportWidth:         constraints.ContentWidth,
		Typography:            TypographyMetrics{FontSize: compilerFontSize},
		CellHorizontalPadding: compilerCellHorizontalPadding,
	})
	c.diagnostics = append(c.diagnostics, diagnostics...)
	if diagnostics.HasErrors() {
		return tableY, nil
	}
	applyCompilerPinning(&layout, constraints)
	c.manifest.HorizontalScroll = layout.HorizontalScroll
	c.horizontalStrategy = compilerHorizontalStrategy(layout, constraints)

	tableRegion := c.input.Blueprint.Regions["table"]
	c.tableRegionID = tableRegion.RootLayerID
	headerHeight := 0.0
	if len(layout.Columns) > 0 {
		headerHeight = constraints.TableHeaderHeight
	}
	tableHeight := headerHeight + float64(len(c.input.PageSpec.Table.SampleRows))*constraints.TableRowHeight
	if err := c.builder.SetBounds(tableRegion.RootLayerID, Rect{
		X: tableRegion.Bounds.X, Y: tableY, Width: constraints.ContentWidth, Height: tableHeight,
	}); err != nil {
		return tableY, err
	}

	if err := c.instantiateTableHeaders(tableRegion, tableY, layout); err != nil {
		return tableY, err
	}
	if err := c.instantiateTableRows(tableRegion, tableY+headerHeight, layout); err != nil {
		return tableY, err
	}
	return tableY + tableHeight, nil
}

func (c *listPageCompiler) instantiateTableHeaders(region BlueprintRegion, tableY float64, layout TableLayout) error {
	prototype := c.input.Blueprint.Prototypes["tableHeaderCell"]
	for index, columnLayout := range layout.Columns {
		column := c.input.PageSpec.Table.Columns[index]
		pinned := columnLayout.Pinned
		bounds := Rect{
			X:      region.Bounds.X + columnLayout.X,
			Y:      tableY,
			Width:  columnLayout.Width,
			Height: c.input.Blueprint.Constraints.TableHeaderHeight,
		}
		container, err := c.builder.CloneSubtree(c.templateSource, prototype.RootLayerID, region.RootLayerID, c.input.Blueprint.FrameID, bounds)
		if err != nil {
			return fmt.Errorf("clone header container %q: %w", column.Key, err)
		}
		c.markGeneratedPrototype(container.RootLayerID, "table-header-cell", "table.columns."+column.Key+".container", pinned)
		if err := c.builder.ClearChildren(container.RootLayerID); err != nil {
			return fmt.Errorf("clear header container %q: %w", column.Key, err)
		}
		_, err = c.instantiateComponent(
			container.RootLayerID,
			bounds,
			RecipeRequest{Kind: "table-header", Variant: "default", State: "default"},
			componentBindingPlan{
				Values:      map[string]string{"label": column.Title, "value": column.Title},
				RequiredAny: []string{"label", "value"},
			},
			"table-header",
			"table.columns."+column.Key,
			pinned,
		)
		if err != nil {
			return fmt.Errorf("instantiate header %q: %w", column.Key, err)
		}
	}
	return nil
}

func (c *listPageCompiler) instantiateTableRows(region BlueprintRegion, firstRowY float64, layout TableLayout) error {
	prototype := c.input.Blueprint.Prototypes["tableRow"]
	rowWidth := math.Max(layout.TotalWidth, c.input.Blueprint.Constraints.ContentWidth)
	for rowIndex, row := range c.input.PageSpec.Table.SampleRows {
		rowY := firstRowY + float64(rowIndex)*c.input.Blueprint.Constraints.TableRowHeight
		rowPath := fmt.Sprintf("table.sampleRows.%d", rowIndex)
		containerBounds := Rect{
			X: region.Bounds.X, Y: rowY, Width: rowWidth, Height: c.input.Blueprint.Constraints.TableRowHeight,
		}
		container, err := c.builder.CloneSubtree(c.templateSource, prototype.RootLayerID, region.RootLayerID, c.input.Blueprint.FrameID, containerBounds)
		if err != nil {
			return fmt.Errorf("clone row container %d: %w", rowIndex, err)
		}
		c.markGeneratedPrototype(container.RootLayerID, "table-row", rowPath, "")
		if err := c.builder.ClearChildren(container.RootLayerID); err != nil {
			return fmt.Errorf("clear row container %d: %w", rowIndex, err)
		}

		rowVariant := "default"
		if rowIndex%2 == 1 {
			rowVariant = "alternate"
		}
		for columnIndex, columnLayout := range layout.Columns {
			column := c.input.PageSpec.Table.Columns[columnIndex]
			value := row[column.Key]
			path := rowPath + "." + column.Key
			bounds := Rect{
				X:      region.Bounds.X + columnLayout.X,
				Y:      rowY,
				Width:  columnLayout.Width,
				Height: c.input.Blueprint.Constraints.TableRowHeight,
			}
			request := RecipeRequest{Kind: "table-row", Variant: rowVariant, State: "default"}
			role := "table-cell"
			requireExactRecipe := false
			if column.Cell == "status-tag" {
				statusVariant, ok := column.StatusMap[value]
				if !ok {
					c.addError("missing_status_mapping", fmt.Sprintf("status %q has no mapping", value), path)
					return nil
				}
				request = RecipeRequest{Kind: "status-tag", Variant: statusVariant, State: "default"}
				role = "status-tag"
				requireExactRecipe = true
			}
			bindings := componentBindingPlan{
				Values:      map[string]string{"label": value, "value": value},
				RequiredAny: []string{"value", "label"},
			}
			var err error
			if requireExactRecipe {
				_, err = c.instantiateExactRecipeComponent(container.RootLayerID, bounds, request, bindings, role, path, columnLayout.Pinned)
			} else {
				_, err = c.instantiateComponent(container.RootLayerID, bounds, request, bindings, role, path, columnLayout.Pinned)
			}
			if err != nil {
				return fmt.Errorf("instantiate row %d column %q: %w", rowIndex, column.Key, err)
			}
		}
		if err := c.instantiateRowActions(container.RootLayerID, region, rowY, rowIndex, layout); err != nil {
			return err
		}
	}
	return nil
}

func (c *listPageCompiler) instantiateRowActions(parentID string, region BlueprintRegion, rowY float64, rowIndex int, layout TableLayout) error {
	if layout.ActionColumn == nil || len(c.input.PageSpec.Table.RowActions) == 0 {
		return nil
	}
	buttonWidth := layout.ActionColumn.Width / float64(len(c.input.PageSpec.Table.RowActions))
	for actionIndex, action := range c.input.PageSpec.Table.RowActions {
		path := fmt.Sprintf("table.sampleRows.%d.rowActions.%s", rowIndex, action.Key)
		bounds := Rect{
			X:      region.Bounds.X + layout.ActionColumn.X + float64(actionIndex)*buttonWidth,
			Y:      rowY,
			Width:  buttonWidth,
			Height: c.input.Blueprint.Constraints.TableRowHeight,
		}
		_, err := c.instantiateComponent(
			parentID,
			bounds,
			RecipeRequest{Kind: "text-button", Variant: "default", State: "default"},
			componentBindingPlan{Values: map[string]string{"label": action.Label}, RequiredAll: []string{"label"}},
			"row-action",
			path,
			layout.ActionColumn.Pinned,
		)
		if err != nil {
			return fmt.Errorf("instantiate row %d action %q: %w", rowIndex, action.Key, err)
		}
	}
	return nil
}

func applyCompilerPinning(layout *TableLayout, constraints BlueprintConstraints) {
	if !layout.HorizontalScroll {
		return
	}
	if constraints.PinFirstColumn && len(layout.Columns) > 0 {
		layout.Columns[0].Pinned = "left"
	}
	if constraints.PinActionColumn && layout.ActionColumn != nil {
		layout.ActionColumn.Pinned = "right"
	}
}

func compilerHorizontalStrategy(layout TableLayout, constraints BlueprintConstraints) string {
	if !layout.HorizontalScroll {
		return "none"
	}
	pinFirst := constraints.PinFirstColumn && len(layout.Columns) > 0
	pinAction := constraints.PinActionColumn && layout.ActionColumn != nil
	switch {
	case pinFirst && pinAction:
		return "scroll-pin-first-and-action"
	case pinFirst:
		return "scroll-pin-first"
	case pinAction:
		return "scroll-pin-action"
	default:
		return "scroll"
	}
}

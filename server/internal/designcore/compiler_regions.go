package designcore

import (
	"fmt"
	"sort"
)

func collectTemplateBusinessTexts(document NativeJSON, blueprint TemplateBlueprint) []string {
	texts := make(map[string]struct{})
	for _, key := range replaceableBusinessRegions {
		region, ok := blueprint.Regions[key]
		if !ok {
			continue
		}
		visited := make(map[string]struct{})
		var visit func(string)
		visit = func(layerID string) {
			if _, seen := visited[layerID]; seen {
				return
			}
			visited[layerID] = struct{}{}
			layer, exists := document.Layers[layerID]
			if !exists {
				return
			}
			if text := structuralLayerText(layer); text != "" {
				texts[text] = struct{}{}
			}
			for _, childID := range layer.Children {
				visit(childID)
			}
		}
		visit(region.RootLayerID)
	}
	result := make([]string, 0, len(texts))
	for text := range texts {
		result = append(result, text)
	}
	sort.Strings(result)
	return result
}

func (c *listPageCompiler) clearBusinessRegions() error {
	for _, key := range replaceableBusinessRegions {
		region, ok := c.input.Blueprint.Regions[key]
		if !ok {
			return fmt.Errorf("replaceable region %q is missing", key)
		}
		if err := c.builder.ClearChildren(region.RootLayerID); err != nil {
			return fmt.Errorf("clear %s region: %w", key, err)
		}
	}
	return nil
}

func (c *listPageCompiler) instantiatePageIdentity() error {
	titlePrototype := c.input.Blueprint.Prototypes["pageTitle"]
	titleRegion := c.input.Blueprint.Regions["pageTitle"]
	titleTarget, ok := titlePrototype.Bindings["label"]
	if !ok || titleTarget == "" {
		c.addError("missing_prototype_binding", "pageTitle prototype must declare a label binding", "blueprint.prototypes.pageTitle.bindings.label")
		return nil
	}
	titleClone, err := c.builder.CloneSubtree(c.templateSource, titlePrototype.RootLayerID, titleRegion.RootLayerID, c.input.Blueprint.FrameID, titleRegion.Bounds)
	if err != nil {
		return fmt.Errorf("clone page title: %w", err)
	}
	c.markGeneratedPrototype(titleClone.RootLayerID, "page-title", "page.title", "")
	if err := c.builder.BindText(titleClone, titleTarget, c.input.PageSpec.Page.Title); err != nil {
		return fmt.Errorf("bind page title: %w", err)
	}
	if err := c.builder.FitCloneLayer(titleClone, titleTarget, titleRegion.Bounds); err != nil {
		return fmt.Errorf("fit page title: %w", err)
	}

	breadcrumbPrototype := c.input.Blueprint.Prototypes["breadcrumbItem"]
	breadcrumbRegion := c.input.Blueprint.Regions["breadcrumb"]
	breadcrumbTarget, ok := breadcrumbPrototype.Bindings["label"]
	if !ok || breadcrumbTarget == "" {
		c.addError("missing_prototype_binding", "breadcrumbItem prototype must declare a label binding", "blueprint.prototypes.breadcrumbItem.bindings.label")
		return nil
	}
	for index, label := range c.input.PageSpec.Page.Breadcrumb {
		bounds := breadcrumbPrototype.Bounds
		bounds.X = breadcrumbRegion.Bounds.X + float64(index)*(breadcrumbPrototype.Bounds.Width+c.input.Blueprint.Constraints.HorizontalGap)
		bounds.Y = breadcrumbRegion.Bounds.Y
		clone, cloneErr := c.builder.CloneSubtree(c.templateSource, breadcrumbPrototype.RootLayerID, breadcrumbRegion.RootLayerID, c.input.Blueprint.FrameID, bounds)
		if cloneErr != nil {
			return fmt.Errorf("clone breadcrumb item %d: %w", index, cloneErr)
		}
		c.markGeneratedPrototype(clone.RootLayerID, "breadcrumb-item", fmt.Sprintf("page.breadcrumb.%d", index), "")
		if bindErr := c.builder.BindText(clone, breadcrumbTarget, label); bindErr != nil {
			return fmt.Errorf("bind breadcrumb item %d: %w", index, bindErr)
		}
		if fitErr := c.builder.FitCloneLayer(clone, breadcrumbTarget, bounds); fitErr != nil {
			return fmt.Errorf("fit breadcrumb item %d: %w", index, fitErr)
		}
	}
	return nil
}

func (c *listPageCompiler) bindActiveNavigation() error {
	if c.input.PageSpec.Page.ActiveNavigation == "" {
		return nil
	}
	region, ok := c.input.Blueprint.Regions["navigation"]
	if !ok {
		c.addError("missing_navigation_binding", "active navigation requires a declared navigation region", "blueprint.regions.navigation")
		return nil
	}
	targetID, ok := region.Bindings["label"]
	if !ok || targetID == "" {
		c.addError("missing_navigation_binding", "navigation region must declare a label binding", "blueprint.regions.navigation.bindings.label")
		return nil
	}
	target, exists := c.templateSource.Layers[targetID]
	if !exists || target.Type != "text" || !isNativeDescendantOrSelf(c.templateSource.Layers, targetID, region.RootLayerID) {
		c.addError("invalid_navigation_binding", fmt.Sprintf("navigation label target %q is not a text descendant", targetID), "blueprint.regions.navigation.bindings.label")
		return nil
	}
	c.navigation = &pendingNavigationBinding{LayerID: targetID, Label: c.input.PageSpec.Page.ActiveNavigation}
	return nil
}

func (c *listPageCompiler) instantiateFiltersAndActions() (float64, error) {
	constraints := c.input.Blueprint.Constraints
	filterRegion := c.input.Blueprint.Regions["filters"]
	filterRows := compilerRowCount(len(c.input.PageSpec.Filters), constraints.FilterColumns)
	filterHeight := 0.0
	if filterRows > 0 {
		filterHeight = float64(filterRows)*constraints.FilterRowHeight + float64(filterRows-1)*constraints.VerticalGap
	}
	if err := c.builder.SetBounds(filterRegion.RootLayerID, Rect{
		X: filterRegion.Bounds.X, Y: filterRegion.Bounds.Y, Width: constraints.ContentWidth, Height: filterHeight,
	}); err != nil {
		return 0, err
	}

	columnWidth := (constraints.ContentWidth - float64(constraints.FilterColumns-1)*constraints.HorizontalGap) / float64(constraints.FilterColumns)
	for index, filter := range c.input.PageSpec.Filters {
		row := index / constraints.FilterColumns
		column := index % constraints.FilterColumns
		bounds := Rect{
			X:     filterRegion.Bounds.X + float64(column)*(columnWidth+constraints.HorizontalGap),
			Y:     filterRegion.Bounds.Y + float64(row)*(constraints.FilterRowHeight+constraints.VerticalGap),
			Width: columnWidth, Height: constraints.FilterRowHeight,
		}
		_, err := c.instantiateComponent(
			filterRegion.RootLayerID,
			bounds,
			RecipeRequest{Kind: filter.Control, Variant: "default", State: "default"},
			componentBindingPlan{
				Values:      map[string]string{"label": filter.Label, "value": filter.Placeholder},
				RequiredAll: []string{"label"},
			},
			"filter-control",
			"filters."+filter.Key,
			"",
		)
		if err != nil {
			return 0, fmt.Errorf("instantiate filter %q: %w", filter.Key, err)
		}
	}

	actionRegion := c.input.Blueprint.Regions["pageActions"]
	actionY := filterRegion.Bounds.Y + filterHeight
	if filterHeight > 0 && len(c.input.PageSpec.PageActions) > 0 {
		actionY += constraints.VerticalGap
	}
	if err := c.builder.SetBounds(actionRegion.RootLayerID, Rect{
		X: actionRegion.Bounds.X, Y: actionY, Width: constraints.ContentWidth, Height: 0,
	}); err != nil {
		return 0, err
	}
	actionX := actionRegion.Bounds.X
	actionHeight := 0.0
	for _, action := range c.input.PageSpec.PageActions {
		component, err := c.instantiateComponent(
			actionRegion.RootLayerID,
			Rect{X: actionX, Y: actionY},
			RecipeRequest{Kind: action.Variant + "-button", Variant: "default", State: "default"},
			componentBindingPlan{Values: map[string]string{"label": action.Label}, RequiredAll: []string{"label"}},
			"page-action",
			"pageActions."+action.Key,
			"",
		)
		if err != nil {
			return 0, fmt.Errorf("instantiate page action %q: %w", action.Key, err)
		}
		if component.Bounds.Height > actionHeight {
			actionHeight = component.Bounds.Height
		}
		actionX += component.Bounds.Width + constraints.HorizontalGap
	}
	if err := c.builder.SetBounds(actionRegion.RootLayerID, Rect{
		X: actionRegion.Bounds.X, Y: actionY, Width: constraints.ContentWidth, Height: actionHeight,
	}); err != nil {
		return 0, err
	}

	tableY := filterRegion.Bounds.Y + filterHeight
	if filterHeight > 0 {
		tableY += constraints.VerticalGap
	}
	if actionHeight > 0 {
		if filterHeight > 0 {
			tableY = actionY
		}
		tableY += actionHeight + constraints.VerticalGap
	}
	return tableY, nil
}

func (c *listPageCompiler) instantiatePagination(tableBottom float64) error {
	region := c.input.Blueprint.Regions["pagination"]
	tableLayer := c.builder.Document().Layers[c.input.Blueprint.Regions["table"].RootLayerID]
	paginationY := tableBottom
	if tableLayer.Height > 0 && c.input.PageSpec.Pagination.Enabled {
		paginationY += c.input.Blueprint.Constraints.VerticalGap
	}
	if !c.input.PageSpec.Pagination.Enabled {
		return c.builder.SetBounds(region.RootLayerID, Rect{
			X: region.Bounds.X, Y: paginationY, Width: c.input.Blueprint.Constraints.ContentWidth, Height: 0,
		})
	}

	label := fmt.Sprintf("%d / %d", c.input.PageSpec.Pagination.PageSize, c.input.PageSpec.Pagination.SampleTotal)
	component, err := c.instantiateComponent(
		region.RootLayerID,
		Rect{X: region.Bounds.X, Y: paginationY, Width: c.input.Blueprint.Constraints.ContentWidth},
		RecipeRequest{Kind: "pagination", Variant: "default", State: "default"},
		componentBindingPlan{
			Values:      map[string]string{"label": label, "value": label},
			RequiredAny: []string{"value", "label"},
		},
		"pagination",
		"pagination",
		"",
	)
	if err != nil {
		return err
	}
	return c.builder.SetBounds(region.RootLayerID, Rect{
		X: region.Bounds.X, Y: paginationY, Width: c.input.Blueprint.Constraints.ContentWidth, Height: component.Bounds.Height,
	})
}

func compilerRowCount(itemCount, columnCount int) int {
	if itemCount == 0 {
		return 0
	}
	return (itemCount + columnCount - 1) / columnCount
}

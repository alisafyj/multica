package designcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const PageSpecVersion = "1.0"

type PageSpec struct {
	Version             string                `json:"version"`
	Page                PageIdentity          `json:"page"`
	Filters             []FilterSpec          `json:"filters"`
	PageActions         []ActionSpec          `json:"pageActions"`
	Table               TableSpec             `json:"table"`
	Pagination          PaginationSpec        `json:"pagination"`
	Assumptions         []string              `json:"assumptions"`
	Warnings            []string              `json:"warnings"`
	RequirementCoverage []RequirementCoverage `json:"requirementCoverage"`
}

type PageIdentity struct {
	Type             string   `json:"type"`
	Module           string   `json:"module"`
	Title            string   `json:"title"`
	Breadcrumb       []string `json:"breadcrumb"`
	ActiveNavigation string   `json:"activeNavigation"`
	Density          string   `json:"density"`
}

type FilterSpec struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Control     string `json:"control"`
	Placeholder string `json:"placeholder"`
	Width       string `json:"width"`
}

type ActionSpec struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Variant string `json:"variant"`
}

type TableSpec struct {
	Columns    []TableColumnSpec   `json:"columns"`
	SampleRows []map[string]string `json:"sampleRows"`
	RowActions []ActionSpec        `json:"rowActions"`
}

type TableColumnSpec struct {
	Key       string            `json:"key"`
	Title     string            `json:"title"`
	Cell      string            `json:"cell"`
	Width     string            `json:"width"`
	Align     string            `json:"align"`
	StatusMap map[string]string `json:"statusMap"`
}

type PaginationSpec struct {
	Enabled     bool `json:"enabled"`
	PageSize    int  `json:"pageSize"`
	SampleTotal int  `json:"sampleTotal"`
}

type RequirementCoverage struct {
	RequirementID string   `json:"requirementId"`
	SpecPaths     []string `json:"specPaths"`
}

var allowedPageTypes = map[string]struct{}{
	"list": {},
}

var allowedControls = map[string]struct{}{
	"input":      {},
	"select":     {},
	"date-range": {},
}

var allowedActionVariants = map[string]struct{}{
	"primary":   {},
	"secondary": {},
	"text":      {},
}

var allowedCells = map[string]struct{}{
	"text":       {},
	"number":     {},
	"date":       {},
	"status-tag": {},
}

var allowedWidths = map[string]struct{}{
	"narrow":   {},
	"medium":   {},
	"wide":     {},
	"flexible": {},
}

var allowedAlignments = map[string]struct{}{
	"":       {},
	"left":   {},
	"center": {},
	"right":  {},
}

var allowedDensities = map[string]struct{}{
	"standard": {},
	"compact":  {},
}

var allowedStatusVariants = map[string]struct{}{
	"success":  {},
	"warning":  {},
	"danger":   {},
	"disabled": {},
	"info":     {},
}

var allowedPageFields = map[string]struct{}{
	"type":             {},
	"module":           {},
	"title":            {},
	"breadcrumb":       {},
	"activeNavigation": {},
	"density":          {},
}

func ParsePageSpec(raw []byte) (PageSpec, error) {
	var spec PageSpec
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return PageSpec{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return PageSpec{}, errors.New("page spec must contain exactly one JSON value")
		}
		return PageSpec{}, err
	}

	return spec, nil
}

func ValidatePageSpec(spec PageSpec, requiredRequirementIDs []string) Diagnostics {
	diagnostics := Diagnostics{}
	if spec.Version != PageSpecVersion {
		diagnostics.addError("unsupported_version", fmt.Sprintf("version must be %q", PageSpecVersion), "version")
	}
	if _, ok := allowedPageTypes[spec.Page.Type]; !ok {
		diagnostics.addError("unsupported_page_type", fmt.Sprintf("page type %q is not supported", spec.Page.Type), "page.type")
	}
	if _, ok := allowedDensities[spec.Page.Density]; !ok {
		diagnostics.addError("unsupported_density", fmt.Sprintf("page density %q is not supported", spec.Page.Density), "page.density")
	}
	validateRequiredText(&diagnostics, spec.Page.Module, "page.module")
	validateRequiredText(&diagnostics, spec.Page.Title, "page.title")

	filterKeys := make(map[string]struct{}, len(spec.Filters))
	for _, filter := range spec.Filters {
		path := fmt.Sprintf("filters.%s", filter.Key)
		validateRequiredText(&diagnostics, filter.Key, path+".key")
		validateRequiredText(&diagnostics, filter.Label, path+".label")
		if !addUniqueKey(&diagnostics, filterKeys, filter.Key, path) {
			continue
		}
		if _, ok := allowedControls[filter.Control]; !ok {
			diagnostics.addError("unsupported_control", fmt.Sprintf("filter control %q is not supported", filter.Control), path)
		}
		if _, ok := allowedWidths[filter.Width]; !ok {
			diagnostics.addError("unsupported_width", fmt.Sprintf("filter width %q is not supported", filter.Width), path)
		}
	}

	pageActionKeys := validateActions(&diagnostics, "pageActions", spec.PageActions)
	columnKeys := make(map[string]struct{}, len(spec.Table.Columns))
	if len(spec.Table.Columns) == 0 {
		diagnostics.addError("missing_table_column", "list pages require at least one table column", "table.columns")
	}
	for _, column := range spec.Table.Columns {
		path := fmt.Sprintf("table.columns.%s", column.Key)
		validateRequiredText(&diagnostics, column.Key, path+".key")
		validateRequiredText(&diagnostics, column.Title, path+".title")
		if !addUniqueKey(&diagnostics, columnKeys, column.Key, path) {
			continue
		}
		if _, ok := allowedCells[column.Cell]; !ok {
			diagnostics.addError("unsupported_cell", fmt.Sprintf("table cell %q is not supported", column.Cell), path)
		}
		if _, ok := allowedWidths[column.Width]; !ok {
			diagnostics.addError("unsupported_width", fmt.Sprintf("table column width %q is not supported", column.Width), path)
		}
		if _, ok := allowedAlignments[column.Align]; !ok {
			diagnostics.addError("unsupported_alignment", fmt.Sprintf("table column alignment %q is not supported", column.Align), path)
		}
		for _, variant := range column.StatusMap {
			if _, ok := allowedStatusVariants[variant]; !ok {
				diagnostics.addError("unsupported_status_variant", fmt.Sprintf("status variant %q is not supported", variant), path)
			}
		}
	}
	rowActionKeys := validateActions(&diagnostics, "table.rowActions", spec.Table.RowActions)
	validateSampleRows(&diagnostics, spec.Table.Columns, spec.Table.SampleRows)
	if spec.Pagination.Enabled && (spec.Pagination.PageSize <= 0 || spec.Pagination.SampleTotal < 0) {
		diagnostics.addError("invalid_pagination", "enabled pagination requires a positive page size and a non-negative sample total", "pagination")
	}
	validateRequirementCoverage(&diagnostics, spec.RequirementCoverage, requiredRequirementIDs, filterKeys, pageActionKeys, columnKeys, rowActionKeys)

	return diagnostics
}

func (d *Diagnostics) addError(code, message string, paths ...string) {
	*d = append(*d, Diagnostic{Code: code, Severity: DiagnosticError, Message: message, Paths: paths})
}

func addUniqueKey(diagnostics *Diagnostics, keys map[string]struct{}, key, path string) bool {
	if _, exists := keys[key]; exists {
		diagnostics.addError("duplicate_key", fmt.Sprintf("key %q is duplicated", key), path)
		return false
	}
	keys[key] = struct{}{}
	return true
}

func validateRequiredText(diagnostics *Diagnostics, value, path string) {
	if strings.TrimSpace(value) == "" {
		diagnostics.addError("missing_required_field", fmt.Sprintf("%s must not be blank", path), path)
	}
}

func validateActions(diagnostics *Diagnostics, scope string, actions []ActionSpec) map[string]struct{} {
	keys := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		path := fmt.Sprintf("%s.%s", scope, action.Key)
		validateRequiredText(diagnostics, action.Key, path+".key")
		validateRequiredText(diagnostics, action.Label, path+".label")
		if !addUniqueKey(diagnostics, keys, action.Key, path) {
			continue
		}
		if _, ok := allowedActionVariants[action.Variant]; !ok {
			diagnostics.addError("unsupported_action_variant", fmt.Sprintf("action variant %q is not supported", action.Variant), path)
		}
	}
	return keys
}

func validateSampleRows(diagnostics *Diagnostics, columns []TableColumnSpec, rows []map[string]string) {
	for rowIndex, row := range rows {
		for _, column := range columns {
			value, exists := row[column.Key]
			if !exists {
				diagnostics.addError("incomplete_sample_row", fmt.Sprintf("sample row %d is missing column %q", rowIndex, column.Key), fmt.Sprintf("table.sampleRows.%d.%s", rowIndex, column.Key))
				continue
			}
			if column.Cell == "status-tag" {
				if _, ok := column.StatusMap[value]; !ok {
					diagnostics.addError("missing_status_mapping", fmt.Sprintf("status %q has no mapping", value), fmt.Sprintf("table.sampleRows.%d.%s", rowIndex, column.Key))
				}
			}
		}
	}
}

func validateRequirementCoverage(diagnostics *Diagnostics, coverage []RequirementCoverage, requiredIDs []string, filterKeys, pageActionKeys, columnKeys, rowActionKeys map[string]struct{}) {
	declaredIDs := make(map[string]struct{}, len(coverage))
	coveredIDs := make(map[string]struct{}, len(coverage))
	for _, item := range coverage {
		path := fmt.Sprintf("requirementCoverage.%s", item.RequirementID)
		validateRequiredText(diagnostics, item.RequirementID, path+".requirementId")
		if !addUniqueKey(diagnostics, declaredIDs, item.RequirementID, path) {
			continue
		}
		validPaths := make(map[string]struct{}, len(item.SpecPaths))
		for _, specPath := range item.SpecPaths {
			if !isValidCoveragePath(specPath, filterKeys, pageActionKeys, columnKeys, rowActionKeys) {
				diagnostics.addError("invalid_spec_path", fmt.Sprintf("spec path %q is not valid", specPath), path)
				continue
			}
			if _, duplicate := validPaths[specPath]; duplicate {
				diagnostics.addError("duplicate_spec_path", fmt.Sprintf("spec path %q is duplicated", specPath), path)
				continue
			}
			validPaths[specPath] = struct{}{}
		}
		if len(validPaths) > 0 {
			coveredIDs[item.RequirementID] = struct{}{}
		}
	}

	for _, requirementID := range requiredIDs {
		if _, covered := coveredIDs[requirementID]; !covered {
			diagnostics.addError("missing_requirement_coverage", fmt.Sprintf("requirement %q has no coverage", requirementID), "requirementCoverage")
		}
	}
}

func isValidCoveragePath(path string, filterKeys, pageActionKeys, columnKeys, rowActionKeys map[string]struct{}) bool {
	switch {
	case path == "pagination":
		return true
	case hasCoveragePathPrefix(path, "filters."):
		_, ok := filterKeys[path[len("filters."):]]
		return ok
	case hasCoveragePathPrefix(path, "pageActions."):
		_, ok := pageActionKeys[path[len("pageActions."):]]
		return ok
	case hasCoveragePathPrefix(path, "table.columns."):
		_, ok := columnKeys[path[len("table.columns."):]]
		return ok
	case hasCoveragePathPrefix(path, "table.rowActions."):
		_, ok := rowActionKeys[path[len("table.rowActions."):]]
		return ok
	case hasCoveragePathPrefix(path, "page."):
		_, ok := allowedPageFields[path[len("page."):]]
		return ok
	default:
		return false
	}
}

func hasCoveragePathPrefix(path, prefix string) bool {
	return len(path) > len(prefix) && path[:len(prefix)] == prefix
}

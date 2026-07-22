package designcore

import "testing"

func TestParseAndValidatePageSpecAcceptsCompleteListPage(t *testing.T) {
	raw := []byte(`{
	  "version":"1.0",
	  "page":{"type":"list","module":"客户管理","title":"客户档案","breadcrumb":["客户管理","客户档案"],"activeNavigation":"客户信息","density":"standard"},
	  "filters":[
	    {"key":"keyword","label":"客户姓名/手机号","control":"input","placeholder":"请输入客户姓名或手机号","width":"medium"},
	    {"key":"status","label":"客户状态","control":"select","placeholder":"请选择客户状态","width":"narrow"},
	    {"key":"createdAt","label":"创建时间","control":"date-range","placeholder":"请选择创建时间","width":"wide"}
	  ],
	  "pageActions":[{"key":"create","label":"新增客户","variant":"primary"}],
	  "table":{"columns":[
	    {"key":"customerName","title":"客户姓名","cell":"text","width":"medium"},
	    {"key":"phone","title":"手机号","cell":"text","width":"medium"},
	    {"key":"status","title":"客户状态","cell":"status-tag","width":"narrow","statusMap":{"正常":"success","待跟进":"warning"}},
	    {"key":"createdAt","title":"创建时间","cell":"date","width":"wide"}
	  ],"sampleRows":[{"customerName":"示例客户A","phone":"13800000001","status":"正常","createdAt":"2026-07-22 10:00"}],"rowActions":[{"key":"view","label":"查看","variant":"text"}]},
	  "pagination":{"enabled":true,"pageSize":20,"sampleTotal":126},
	  "assumptions":[],"warnings":[],
	  "requirementCoverage":[
	    {"requirementId":"filter-keyword","specPaths":["filters.keyword"]},
	    {"requirementId":"filter-status","specPaths":["filters.status"]},
	    {"requirementId":"filter-created-at","specPaths":["filters.createdAt"]}
	  ]
	}`)
	spec, err := ParsePageSpec(raw)
	if err != nil {
		t.Fatalf("ParsePageSpec: %v", err)
	}
	diagnostics := ValidatePageSpec(spec, []string{"filter-keyword", "filter-status", "filter-created-at"})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
}

func TestParsePageSpecRejectsLayerAndGeometryFields(t *testing.T) {
	tests := []struct {
		name      string
		pageField string
		field     string
	}{
		{name: "layer ID", field: `,"layerId":"figma-1"`},
		{name: "page geometry", pageField: `,"x":20`},
		{name: "patch operations", field: `,"patchOperations":[]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(`{"version":"1.0","page":{"type":"list","module":"CRM","title":"客户","density":"standard"` + tt.pageField + `},"filters":[],"pageActions":[],"table":{"columns":[],"sampleRows":[],"rowActions":[]},"pagination":{"enabled":false}` + tt.field + `}`)
			if _, err := ParsePageSpec(raw); err == nil {
				t.Fatal("expected unknown semantic fields to be rejected")
			}
		})
	}
}

func TestParsePageSpecRejectsTrailingJSON(t *testing.T) {
	_, err := ParsePageSpec([]byte(`{"version":"1.0","page":{"type":"list"}} {}`))
	if err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func TestValidatePageSpecReportsSemanticContractDiagnostics(t *testing.T) {
	tests := []struct {
		name           string
		spec           PageSpec
		requirementIDs []string
		codes          []string
	}{
		{
			name: "duplicate filter column and action keys",
			spec: func() PageSpec {
				spec := completePageSpec()
				spec.Filters = append(spec.Filters, spec.Filters[0])
				spec.Table.Columns = append(spec.Table.Columns, spec.Table.Columns[0])
				spec.PageActions = append(spec.PageActions, spec.PageActions[0])
				spec.Table.RowActions = append(spec.Table.RowActions, spec.Table.RowActions[0])
				return spec
			}(),
			codes: []string{"duplicate_key"},
		},
		{
			name: "unsupported page control and cell values",
			spec: func() PageSpec {
				spec := completePageSpec()
				spec.Page.Type = "detail"
				spec.Filters[0].Control = "checkbox"
				spec.Table.Columns[0].Cell = "avatar"
				return spec
			}(),
			codes: []string{"unsupported_page_type", "unsupported_control", "unsupported_cell"},
		},
		{
			name: "unmapped sample row status",
			spec: func() PageSpec {
				spec := completePageSpec()
				spec.Table.SampleRows[0]["status"] = "已停用"
				return spec
			}(),
			codes: []string{"missing_status_mapping"},
		},
		{
			name: "sample row misses visible column",
			spec: func() PageSpec {
				spec := completePageSpec()
				delete(spec.Table.SampleRows[0], "phone")
				return spec
			}(),
			codes: []string{"incomplete_sample_row"},
		},
		{
			name: "duplicate and missing requirement coverage",
			spec: func() PageSpec {
				spec := completePageSpec()
				spec.RequirementCoverage = append(spec.RequirementCoverage, spec.RequirementCoverage[0])
				return spec
			}(),
			requirementIDs: []string{"filter-keyword", "filter-status", "filter-created-at", "page-create"},
			codes:          []string{"duplicate_key", "missing_requirement_coverage"},
		},
		{
			name: "invalid requirement coverage paths",
			spec: func() PageSpec {
				spec := completePageSpec()
				spec.RequirementCoverage[0].SpecPaths = []string{"/filters/keyword", "table.columns.missing"}
				return spec
			}(),
			codes: []string{"invalid_spec_path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := ValidatePageSpec(tt.spec, tt.requirementIDs)
			for _, code := range tt.codes {
				assertDiagnosticCode(t, diagnostics, code)
			}
		})
	}
}

func TestValidatePageSpecRejectsSemanticallyEmptyListShape(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PageSpec)
		code   string
	}{
		{name: "blank module", mutate: func(spec *PageSpec) { spec.Page.Module = " \t" }, code: "missing_required_field"},
		{name: "blank title", mutate: func(spec *PageSpec) { spec.Page.Title = "" }, code: "missing_required_field"},
		{name: "blank filter key", mutate: func(spec *PageSpec) { spec.Filters[0].Key = " " }, code: "missing_required_field"},
		{name: "blank filter label", mutate: func(spec *PageSpec) { spec.Filters[0].Label = "" }, code: "missing_required_field"},
		{name: "blank action key", mutate: func(spec *PageSpec) { spec.PageActions[0].Key = "" }, code: "missing_required_field"},
		{name: "blank action label", mutate: func(spec *PageSpec) { spec.Table.RowActions[0].Label = "\n" }, code: "missing_required_field"},
		{name: "no columns", mutate: func(spec *PageSpec) { spec.Table.Columns = nil }, code: "missing_table_column"},
		{name: "blank column key", mutate: func(spec *PageSpec) { spec.Table.Columns[0].Key = "" }, code: "missing_required_field"},
		{name: "blank column title", mutate: func(spec *PageSpec) { spec.Table.Columns[0].Title = " " }, code: "missing_required_field"},
		{name: "enabled pagination without page size", mutate: func(spec *PageSpec) { spec.Pagination.PageSize = 0 }, code: "invalid_pagination"},
		{name: "enabled pagination with negative total", mutate: func(spec *PageSpec) { spec.Pagination.SampleTotal = -1 }, code: "invalid_pagination"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := completePageSpec()
			tt.mutate(&spec)
			assertDiagnosticCode(t, ValidatePageSpec(spec, nil), tt.code)
		})
	}
}

func TestValidatePageSpecRequiresRealRequirementCoveragePaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		codes []string
	}{
		{name: "empty", paths: nil, codes: []string{"missing_requirement_coverage"}},
		{name: "blank", paths: []string{"", "  "}, codes: []string{"invalid_spec_path", "missing_requirement_coverage"}},
		{name: "invalid", paths: []string{"filters.missing"}, codes: []string{"invalid_spec_path", "missing_requirement_coverage"}},
		{name: "duplicate", paths: []string{"filters.keyword", "filters.keyword"}, codes: []string{"duplicate_spec_path"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := completePageSpec()
			spec.RequirementCoverage = []RequirementCoverage{{RequirementID: "REQ-1", SpecPaths: tt.paths}}
			diagnostics := ValidatePageSpec(spec, []string{"REQ-1"})
			for _, code := range tt.codes {
				assertDiagnosticCode(t, diagnostics, code)
			}
		})
	}
}

func TestDiagnosticsHasErrors(t *testing.T) {
	if (Diagnostics{{Severity: DiagnosticWarning}}).HasErrors() {
		t.Fatal("warnings must not be errors")
	}
	if !(Diagnostics{{Severity: DiagnosticError}}).HasErrors() {
		t.Fatal("errors must be reported")
	}
}

func completePageSpec() PageSpec {
	return PageSpec{
		Version: PageSpecVersion,
		Page:    PageIdentity{Type: "list", Module: "CRM", Title: "客户", Density: "standard"},
		Filters: []FilterSpec{
			{Key: "keyword", Label: "关键词", Control: "input", Width: "medium"},
			{Key: "status", Label: "状态", Control: "select", Width: "narrow"},
			{Key: "createdAt", Label: "创建时间", Control: "date-range", Width: "wide"},
		},
		PageActions: []ActionSpec{{Key: "create", Label: "新增", Variant: "primary"}},
		Table: TableSpec{
			Columns: []TableColumnSpec{
				{Key: "customerName", Title: "客户姓名", Cell: "text", Width: "medium"},
				{Key: "phone", Title: "手机号", Cell: "text", Width: "medium"},
				{Key: "status", Title: "状态", Cell: "status-tag", Width: "narrow", StatusMap: map[string]string{"正常": "success", "待跟进": "warning"}},
				{Key: "createdAt", Title: "创建时间", Cell: "date", Width: "wide"},
			},
			SampleRows: []map[string]string{{"customerName": "示例客户", "phone": "13800000000", "status": "正常", "createdAt": "2026-07-22"}},
			RowActions: []ActionSpec{{Key: "view", Label: "查看", Variant: "text"}},
		},
		Pagination: PaginationSpec{Enabled: true, PageSize: 20, SampleTotal: 126},
		RequirementCoverage: []RequirementCoverage{
			{RequirementID: "filter-keyword", SpecPaths: []string{"filters.keyword"}},
			{RequirementID: "filter-status", SpecPaths: []string{"filters.status"}},
			{RequirementID: "filter-created-at", SpecPaths: []string{"filters.createdAt"}},
		},
	}
}

func assertDiagnosticCode(t *testing.T, diagnostics Diagnostics, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			return
		}
	}
	t.Fatalf("expected diagnostic code %q, got %+v", want, diagnostics)
}

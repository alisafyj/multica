package opendesign

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPackageAuditAcceptsWarningOnlyStrictFailure(t *testing.T) {
	t.Parallel()

	audit := PackageAudit{
		OK:             false,
		FilesInspected: 39,
		Errors:         []PackageAuditIssue{},
		Warnings: []PackageAuditIssue{{
			Severity: "warning",
			Code:     "readme_missing_product_overview",
			Message:  "README needs a product overview",
			Path:     "README.md",
		}},
	}
	if err := ValidatePackageAudit(audit); err != nil {
		t.Fatalf("ValidatePackageAudit warning-only failure: %v", err)
	}

	var failure map[string]string
	if err := json.Unmarshal(PackageAuditFailure(audit), &failure); err != nil {
		t.Fatalf("decode PackageAuditFailure: %v", err)
	}
	if failure["code"] != "open_design_package_audit_failed" ||
		!strings.Contains(failure["message"], "1 warning(s)") ||
		!strings.Contains(failure["message"], "readme_missing_product_overview") ||
		!strings.Contains(failure["message"], "README.md") {
		t.Fatalf("warning-only failure = %#v", failure)
	}
}

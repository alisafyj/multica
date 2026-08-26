package designdocument

import (
	"path/filepath"
	"strings"
	"testing"
)

func validCritiqueJSON() string {
	return `{
  "schema_version": "multica.design-document-critique/v1",
  "threshold": 8,
  "max_rounds": 3,
  "outcome": "passed",
  "rounds": [
    {
      "index": 1,
      "scores": {"designer": 7, "critic": 6, "brand": 8, "a11y": 5, "copy": 8},
      "findings": [
        {"lens": "a11y", "severity": "must_fix", "summary": "Filter chips have no visible focus ring.", "resolved": true},
        {"lens": "critic", "severity": "should_fix", "summary": "Empty state does not explain how to add an order.", "resolved": true}
      ]
    },
    {
      "index": 2,
      "scores": {"designer": 8, "critic": 8, "brand": 9, "a11y": 8, "copy": 8},
      "findings": []
    }
  ]
}`
}

// critique.json is optional, but when it is present it is validated like the
// other agent documents: strict shape, known lenses and severities, bounded
// rounds and scores, and a coherent outcome. Its scores never change the audit
// verdict (DC-050): a failing critique still yields a passing package.
func TestCritiqueDocumentIsAcceptedAndValidated(t *testing.T) {
	root := copyFixture(t)
	writeFixtureFile(t, root, critiquePath, []byte(validCritiqueJSON()))
	collected, err := CollectDirectory(root, validBinding())
	if err != nil {
		t.Fatalf("CollectDirectory with critique: %v", err)
	}
	var critiqueEntry *ArtifactIndexEntry
	for i := range collected.Manifest.Files {
		if collected.Manifest.Files[i].Path == critiquePath {
			critiqueEntry = &collected.Manifest.Files[i]
		}
	}
	if critiqueEntry == nil || critiqueEntry.Role != "critique" || critiqueEntry.MediaType != "application/json" {
		t.Fatalf("critique entry = %+v", critiqueEntry)
	}
	if !collected.Audit.Passed {
		t.Fatalf("package with a valid critique failed the audit: %+v", collected.Audit.Diagnostics)
	}

	// Low scores and an unresolved must-fix are still a valid, passing
	// package: the critique is a report, not a gate.
	stopped := strings.Replace(validCritiqueJSON(), `"outcome": "passed"`, `"outcome": "stopped_at_max_rounds"`, 1)
	stopped = strings.Replace(stopped, `"resolved": true}`, `"resolved": false}`, 1)
	root = copyFixture(t)
	writeFixtureFile(t, root, critiquePath, []byte(stopped))
	collected, err = CollectDirectory(root, validBinding())
	if err != nil || !collected.Audit.Passed {
		t.Fatalf("a low-scoring critique blocked the package: err=%v audit=%+v", err, collected.Audit.Diagnostics)
	}
}

func TestCritiqueDocumentRejectsMalformedReports(t *testing.T) {
	for _, tt := range []struct {
		name  string
		edit  func(string) string
		code  string
		valid bool
	}{
		{name: "unknown lens", edit: func(s string) string { return strings.Replace(s, `"lens": "a11y"`, `"lens": "vibes"`, 1) }, code: "critique_lens_invalid"},
		{name: "score out of range", edit: func(s string) string { return strings.Replace(s, `"designer": 7`, `"designer": 11`, 1) }, code: "critique_score_invalid"},
		{name: "unknown severity", edit: func(s string) string { return strings.Replace(s, `"must_fix"`, `"blocker"`, 1) }, code: "critique_severity_invalid"},
		{name: "unknown outcome", edit: func(s string) string { return strings.Replace(s, `"passed"`, `"shipped"`, 1) }, code: "critique_outcome_invalid"},
		{name: "wrong schema", edit: func(s string) string { return strings.Replace(s, "critique/v1", "critique/v2", 1) }, code: "critique_schema_invalid"},
		{name: "no rounds", edit: func(s string) string { return strings.Replace(s, `"rounds": [`, `"rounds": [], "ignored": [`, 1) }, code: "critique_invalid"},
		{name: "unknown field", edit: func(s string) string {
			return strings.Replace(s, `"threshold": 8,`, `"threshold": 8, "extra": true,`, 1)
		}, code: "critique_invalid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := copyFixture(t)
			writeFixtureFile(t, root, critiquePath, []byte(tt.edit(validCritiqueJSON())))
			_, err := CollectDirectory(root, validBinding())
			if err == nil {
				t.Fatal("malformed critique was accepted")
			}
			assertErrorContains(t, err, tt.code)
		})
	}
	// A directory that only *looks* like the critique path is still refused.
	root := copyFixture(t)
	writeFixtureFile(t, root, filepath.Join("prototype", "critique.json"), []byte(validCritiqueJSON()))
	if _, err := CollectDirectory(root, validBinding()); err == nil {
		t.Fatal("critique.json under prototype/ was accepted")
	}
}

package main

import (
	"reflect"
	"testing"
)

func TestSplitPathLinesAcceptsDiffAndStatusShapes(t *testing.T) {
	got := splitPathLines("src/a.ts\r\n\n M src/b.ts\nM\tsrc/c.ts\n?? new/file.go\nsrc/d e.ts\n")
	want := []string{"src/a.ts", "src/b.ts", "src/c.ts", "new/file.go", "src/d e.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitPathLines = %q, want %q", got, want)
	}
	if got := dedupePaths([]string{"a", " a ", "", "b", "a"}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("dedupePaths = %q", got)
	}
	ids := recommendedCaseIDs([]any{
		map[string]any{"test_case": map[string]any{"id": "c2"}},
		map[string]any{"test_case": map[string]any{"id": "c1"}},
		map[string]any{"nope": true},
	})
	if !reflect.DeepEqual(ids, []string{"c2", "c1"}) {
		t.Errorf("recommendedCaseIDs = %q, want the server order", ids)
	}
}

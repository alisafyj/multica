package main

import (
	"reflect"
	"testing"
)

func TestFilterDaemonLogNoisePreservesJSONSeverity(t *testing.T) {
	t.Parallel()
	warning := `{"level":"WARN","msg":"keep warning"}`
	errorLine := `{"level":"ERROR","msg":"keep an INF token inside an error"}`
	plain := "plain crash message"
	malformed := `{"level":`
	lines := []string{
		`{"level":"DEBUG","msg":"discard debug"}`,
		`{"level":"INFO","msg":"discard info"}`,
		"18:00:00.001 DBG discard tint debug",
		"18:00:00.002 INF discard tint info",
		warning, errorLine, plain, malformed,
	}
	if got, want := filterDaemonLogNoise(lines, 5), []string{warning, errorLine, plain, malformed}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered excerpt = %v, want %v", got, want)
	}
	if got, want := filterDaemonLogNoise(lines, 2), []string{plain, malformed}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capped excerpt = %v, want %v", got, want)
	}
}

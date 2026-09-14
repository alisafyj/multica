package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitNexusStatusFixture(root string) map[string]any {
	return map[string]any{
		"schemaVersion": 1,
		"repository":    root,
		"status":        "up-to-date",
		"index": map[string]any{
			"commit":               strings.Repeat("a", 40),
			"runnerIdentityStatus": "current",
			"incompleteReasons":    []string{},
		},
		"current":      map[string]any{"commit": strings.Repeat("a", 40)},
		"contentDrift": map[string]any{"status": "current", "coveredFiles": 12},
	}
}

func TestGitNexusReadinessRequiresCompleteEvidence(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{name: "complete evidence", want: "ready"},
		{
			name:   "foreign repository",
			change: func(s map[string]any) { s["repository"] = filepath.Join(root, "another-checkout") },
			want:   "failed",
		},
		{
			name:   "revision changed",
			change: func(s map[string]any) { s["current"].(map[string]any)["commit"] = strings.Repeat("b", 40) },
			want:   "stale",
		},
		{
			name: "both revisions unknown",
			change: func(s map[string]any) {
				s["current"].(map[string]any)["commit"] = ""
				s["index"].(map[string]any)["commit"] = ""
			},
			want: "stale",
		},
		{
			name:   "missing analyzer verdict",
			change: func(s map[string]any) { delete(s["index"].(map[string]any), "runnerIdentityStatus") },
			want:   "unsupported",
		},
		{
			name:   "stale analyzer",
			change: func(s map[string]any) { s["index"].(map[string]any)["runnerIdentityStatus"] = "stale-or-unknown" },
			want:   "stale",
		},
		{
			name: "incomplete index",
			change: func(s map[string]any) {
				s["index"].(map[string]any)["incompleteReasons"] = []string{"missing-coverage"}
			},
			want: "stale",
		},
		{
			name:   "missing completeness verdict",
			change: func(s map[string]any) { delete(s["index"].(map[string]any), "incompleteReasons") },
			want:   "unsupported",
		},
		{
			name:   "missing content drift",
			change: func(s map[string]any) { delete(s, "contentDrift") },
			want:   "unsupported",
		},
		{
			name: "unmeasurable clean legacy index",
			change: func(s map[string]any) {
				s["contentDrift"] = map[string]any{"status": "unmeasurable", "reason": "no-file-hashes"}
			},
			want: "stale",
		},
		{
			name:   "content changed at same revision",
			change: func(s map[string]any) { s["contentDrift"] = map[string]any{"status": "drifted"} },
			want:   "stale",
		},
		{
			name:   "content not checked",
			change: func(s map[string]any) { s["contentDrift"] = map[string]any{"status": "not-checked"} },
			want:   "stale",
		},
		{
			name:   "missing measured coverage",
			change: func(s map[string]any) { delete(s["contentDrift"].(map[string]any), "coveredFiles") },
			want:   "unsupported",
		},
		{
			name:   "upstream stale verdict remains authoritative",
			change: func(s map[string]any) { s["status"] = "stale" },
			want:   "stale",
		},
		{
			name:   "newer schema",
			change: func(s map[string]any) { s["schemaVersion"] = 2 },
			want:   "unsupported",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := gitNexusStatusFixture(root)
			if tt.change != nil {
				tt.change(payload)
			}
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			status, reason := classifyGitNexusStatus(data, root)
			if status != tt.want {
				t.Fatalf("status = %s (%s), want %s", status, reason, tt.want)
			}
		})
	}
}

func TestGitNexusStatusRejectsInvalidStructuredOutput(t *testing.T) {
	root := t.TempDir()
	for _, data := range []string{
		"GitNexus index is up-to-date",
		`{"schemaVersion":1} {"schemaVersion":1}`,
		`{"schemaVersion":"1"}`,
		`null`,
	} {
		status, reason := classifyGitNexusStatus([]byte(data), root)
		if status != "unsupported" {
			t.Errorf("status = %s (%s), want unsupported for %q", status, reason, data)
		}
	}
}

func TestGitNexusMissingAndLegacyIndexes(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		upstream string
		want     string
	}{
		{upstream: "not-indexed", want: "not_indexed"},
		{upstream: "stale-kuzu-index", want: "stale"},
	} {
		t.Run(tt.upstream, func(t *testing.T) {
			data, err := json.Marshal(map[string]any{"schemaVersion": 1, "repository": root, "error": tt.upstream})
			if err != nil {
				t.Fatal(err)
			}
			status, reason := classifyGitNexusStatus(data, root)
			if status != tt.want {
				t.Fatalf("status = %s (%s), want %s", status, reason, tt.want)
			}
		})
	}
}

const repoToolStatusChildEnv = "MULTICA_TEST_REPO_TOOL_STATUS_CHILD"

// Like the daemon identity tests, re-execute this test binary so the fixture
// works without a platform-specific shell or any installed GitNexus binary.
func TestRepoToolStatusChildProcess(t *testing.T) {
	scenario := os.Getenv(repoToolStatusChildEnv)
	if scenario == "" {
		return
	}
	switch scenario {
	case "timeout":
		time.Sleep(30 * time.Second)
	case "output limit":
		fmt.Fprint(os.Stdout, strings.Repeat("x", repoToolStatusOutputLimit+1))
	case "unsupported option":
		fmt.Fprint(os.Stderr, "private diagnostic: unknown option '--json'; private path")
		os.Exit(1)
	case "failed status":
		root, err := os.Getwd()
		if err != nil {
			os.Exit(2)
		}
		_ = json.NewEncoder(os.Stdout).Encode(gitNexusStatusFixture(root))
		fmt.Fprint(os.Stderr, "private diagnostic that must not escape")
		os.Exit(1)
	}
	os.Exit(0)
}

func TestRepoToolStatusBoundsAndSanitizesSubprocess(t *testing.T) {
	for _, tt := range []struct {
		name       string
		timeout    time.Duration
		wantReason string
	}{
		{name: "timeout", timeout: 100 * time.Millisecond, wantReason: "timed_out"},
		{name: "output limit", timeout: 5 * time.Second, wantReason: "output_limit_exceeded"},
		{name: "unsupported option", timeout: 5 * time.Second, wantReason: "json_status_unsupported"},
		{name: "failed status", timeout: 5 * time.Second, wantReason: "status_command_failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestRepoToolStatusChildProcess$")
			command.Dir = t.TempDir()
			command.Env = []string{repoToolStatusChildEnv + "=" + tt.name}
			started := time.Now()
			data, reason := captureRepoToolStatus(context.Background(), command, tt.timeout)
			if reason != tt.wantReason {
				t.Fatalf("reason = %s, want %s", reason, tt.wantReason)
			}
			if len(data) != 0 {
				t.Fatal("failed subprocess exposed its output")
			}
			if elapsed := time.Since(started); elapsed > tt.timeout+3*time.Second {
				t.Fatalf("subprocess was not bounded: %s", elapsed)
			}
		})
	}
}

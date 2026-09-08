//go:build !windows

package daemon

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskCapabilitySpecRequiresMatchingTask(t *testing.T) {
	root := t.TempDir()
	file := func(name string) pairT3File {
		return pairT3File{Path: filepath.Join(root, name), SHA256: pairT3SHA(name)}
	}
	spec := pairT3Spec{
		SchemaVersion: "codex_capability_t4_spec/v1", CollectorClassification: "synthetic_test_diagnostic",
		Registration: file("registration"), ProductCodeCheck: file("checker"),
		Runner:             pairT3Runner{Path: filepath.Join(root, "runner"), SHA256: pairT3SHA("runner")},
		SyntheticTargetRef: "outside-owned-sentinel", EvidenceOutput: filepath.Join(root, "evidence"), PnpmVersion: pairT3PnpmVersion,
	}
	locks, _ := json.Marshal(pairT3SourceLocks(root))
	if err := json.Unmarshal(locks, &spec.Runner.Sources); err != nil {
		t.Fatal(err)
	}
	spec.Source.BareRepository = filepath.Join(root, "source.git")
	spec.Source.Revision = strings.Repeat("a", 40)
	spec.Source.TreeSHA256 = pairT3SHA("tree")
	spec.Source.DependencyFiles = []pairT3File{{Path: "go.mod", SHA256: pairT3SHA("go.mod")}}
	spec.Arms.Native = pairT3Arm{ArmID: "native", Nonce: strings.Repeat("1", 32), SandboxPolicy: pairT3SandboxPolicy{SHA256: pairT3SHA("policy"), NetworkAccess: true}}
	spec.Arms.Managed = pairT3Arm{ArmID: "patched_ordinary", Nonce: strings.Repeat("2", 32), SandboxPolicy: spec.Arms.Native.SandboxPolicy}
	spec.Toolchain.Node, spec.Toolchain.Git, spec.Toolchain.Go, spec.Toolchain.Pnpm = file("node"), file("git"), file("go"), file("pnpm")
	spec.CacheSeeds.GoMod = pairT3Directory{Path: filepath.Join(root, "gomod"), SHA256: pairT3SHA("gomod")}
	spec.CacheSeeds.PnpmStore = pairT3Directory{Path: filepath.Join(root, "pnpm"), SHA256: pairT3SHA("pnpm")}
	spec.CacheSeeds.CorepackBundle = pairT3Directory{Path: filepath.Join(root, "corepack"), SHA256: pairT3SHA("corepack")}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseCodexTaskCapabilitySpec(data, "T4"); err != nil {
		t.Fatalf("matching T4 spec rejected: %v", err)
	}
	for _, task := range []string{"T3", "T5", "", "t4"} {
		if _, err := parseCodexTaskCapabilitySpec(data, task); err == nil {
			t.Fatalf("T4 spec accepted through %q selector", task)
		}
	}
	if _, err := parseCodexT3CapabilitySpec(data); err == nil {
		t.Fatal("legacy T3 parser accepted a T4 spec")
	}
	var unknown map[string]any
	if err := json.Unmarshal(data, &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["task_id"] = "T4"
	data, _ = json.Marshal(unknown)
	if _, err := parseCodexTaskCapabilitySpec(data, "T4"); err == nil {
		t.Fatal("unknown spec field accepted")
	}
}

func TestTaskCapabilityRegistrationRequiresProspectiveSingleTask(t *testing.T) {
	for _, tc := range []struct {
		name, schema, task string
		ids                []string
		valid              bool
	}{
		{"t4", "treatment-aware-task-scoped/v2", "T4", []string{"T4"}, true},
		{"t3-v1", "treatment-aware-task-scoped/v1", "T3", []string{"T3"}, true},
		{"t3-v2", "treatment-aware-task-scoped/v2", "T3", []string{"T3"}, true},
		{"t4-v1", "treatment-aware-task-scoped/v1", "T4", []string{"T4"}, false},
		{"wrong-task", "treatment-aware-task-scoped/v2", "T4", []string{"T3"}, false},
		{"mixed", "treatment-aware-task-scoped/v2", "T4", []string{"T3", "T4"}, false},
		{"missing", "treatment-aware-task-scoped/v2", "T4", nil, false},
		{"unknown-task", "treatment-aware-task-scoped/v2", "T5", []string{"T5"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"schema_version": tc.schema, "schedule": map[string]any{"task_ids": tc.ids}})
			if err := pairValidateTaskCapabilityRegistration(data, tc.task); (err == nil) != tc.valid {
				t.Fatalf("valid=%t error=%v", tc.valid, err)
			}
		})
	}
}

func TestTaskCapabilityToolOutputRejectsCrossTaskMarker(t *testing.T) {
	request := map[string]any{"input": []any{map[string]any{"type": "function_call_output", "output": "Process exited with code 0\nT4_CAPABILITY_PROBE_RECEIPT " + pairT3SHA("receipt")}}}
	if ok, digest := pairTaskToolOutputProof(request, "T4"); !ok || digest != pairT3SHA("receipt") {
		t.Fatal("valid T4 marker rejected")
	}
	for _, task := range []string{"T3", "T5", ""} {
		if ok, _ := pairTaskToolOutputProof(request, task); ok {
			t.Fatalf("T4 marker accepted through %q", task)
		}
	}
}

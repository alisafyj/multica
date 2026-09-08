//go:build !windows

package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPairT3VerifyTrackedSourceRejectsTrackedDriftButAllowsCaches(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		output, runErr := cmd.CombinedOutput()
		if runErr != nil {
			t.Fatalf("git %v: %v (%s)", args, runErr, output)
		}
		return strings.TrimSpace(string(output))
	}
	run("init", "--quiet")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("frozen\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("-c", "user.name=T3 Test", "-c", "user.email=t3@example.invalid", "commit", "--quiet", "-m", "fixture")

	var spec pairT3Spec
	spec.Toolchain.Git.Path = gitPath
	spec.Source.BareRepository = filepath.Join(root, ".git")
	spec.Source.Revision = run("rev-parse", "HEAD")
	if err := pairT3VerifyTrackedSource(spec, root); err != nil {
		t.Fatalf("clean source rejected: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".capability-cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".capability-cache", "mutable"), []byte("allowed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pairT3VerifyTrackedSource(spec, root); err != nil {
		t.Fatalf("untracked cache rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pairT3VerifyTrackedSource(spec, root); err == nil {
		t.Fatal("tracked source drift was accepted")
	}
	run("checkout", "--", "tracked.txt")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("staged drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	if err := pairT3VerifyTrackedSource(spec, root); err == nil {
		t.Fatal("staged source drift was accepted")
	}
}

func TestPairT3HashTreeUsesPinnedNodeLocaleOrderForCaseAndPunctuation(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	root := t.TempDir()
	for name, content := range map[string]string{"B": "upper\n", "_a": "underscore\n", "a": "lower\n", ".a": "dot\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	helper := filepath.Join(t.TempDir(), "native-capture.mjs")
	module := `import {createHash} from "node:crypto"; import {readdir,readFile} from "node:fs/promises"; import path from "node:path";
export async function hashDirectoryTree(root) { const files=(await readdir(root)).sort((a,b)=>a.localeCompare(b)); const hash=createHash("sha256"); let bytes=0; for (const file of files) { const data=await readFile(path.join(root,file)); hash.update(file).update("\\0").update(data).update("\\0"); bytes+=data.length; } return {sha256:"sha256:"+hash.digest("hex"),file_count:files.length,bytes}; }
`
	if err := os.WriteFile(helper, []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err := pairT3HashTreeWithNode(nodePath, helper, root)
	if err != nil {
		t.Fatal(err)
	}
	if observed.SHA256 != "sha256:805c6919a02a01723ec2251489fda4fb721b7c388d717835d7c1dfdce9cf7fb2" || observed.FileCount != 4 || observed.Bytes != 27 {
		t.Fatalf("unexpected locale-ordered tree digest: %+v", observed)
	}
}

func TestParseCodexT3CapabilitySpecRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	valid := map[string]any{
		"schema_version":           "codex_capability_t3_spec/v1",
		"collector_classification": "synthetic_test_diagnostic",
		"registration":             map[string]any{"path": filepath.Join(root, "registration.json"), "sha256": pairT3SHA("registration")},
		"product_code_check":       map[string]any{"path": filepath.Join(root, "product-check.mjs"), "sha256": pairT3SHA("product-check")},
		"runner":                   map[string]any{"path": filepath.Join(root, "runner.mjs"), "sha256": pairT3SHA("runner"), "sources": pairT3SourceLocks(root)},
		"source": map[string]any{
			"bare_repository": filepath.Join(root, "source.git"), "revision": pairT3Repeat('a', 40), "tree_sha256": pairT3SHA("tree"),
			"dependency_files": []any{map[string]any{"path": "go.mod", "sha256": pairT3SHA("go.mod")}},
		},
		"arms": map[string]any{
			"native":  map[string]any{"arm_id": "native", "nonce": pairT3Repeat('1', 32), "sandbox_policy": pairT3TestPolicy()},
			"managed": map[string]any{"arm_id": "managed", "nonce": pairT3Repeat('2', 32), "sandbox_policy": pairT3TestPolicy()},
		},
		"toolchain": map[string]any{
			"node": map[string]any{"path": filepath.Join(root, "node"), "sha256": pairT3SHA("node")},
			"git":  map[string]any{"path": filepath.Join(root, "git"), "sha256": pairT3SHA("git")},
			"go":   map[string]any{"path": filepath.Join(root, "go"), "sha256": pairT3SHA("go")},
			"pnpm": map[string]any{"path": filepath.Join(root, "pnpm"), "sha256": pairT3SHA("pnpm")},
		},
		"synthetic_target_ref": "outside-owned-sentinel",
		"evidence_output":      filepath.Join(root, "evidence.json"),
		"cache_seeds": map[string]any{
			"go_mod":          map[string]any{"path": filepath.Join(root, "gomod"), "sha256": pairT3SHA("gomod"), "file_count": 0, "bytes": 0},
			"pnpm_store":      map[string]any{"path": filepath.Join(root, "pnpm-store"), "sha256": pairT3SHA("pnpm-store"), "file_count": 0, "bytes": 0},
			"corepack_bundle": map[string]any{"path": filepath.Join(root, "corepack"), "sha256": pairT3SHA("corepack"), "file_count": 1, "bytes": 10},
		},
		"pnpm_version": "10.28.2",
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseCodexT3CapabilitySpec(data); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	valid["unknown"] = true
	data, _ = json.Marshal(valid)
	if _, err := parseCodexT3CapabilitySpec(data); err == nil {
		t.Fatal("unknown top-level field accepted")
	}
}

func pairT3TestPolicy() map[string]any {
	return map[string]any{"sha256": pairT3SHA("policy"), "network_access": true, "exclude_tmpdir_env_var": false, "exclude_slash_tmp": false}
}

func TestCodexT3CapabilityWrapperUsesPrivateCorepackHome(t *testing.T) {
	root := t.TempDir()
	workspaces := [2]pairWorkspace{
		{cwd: filepath.Join(root, "native")},
		{cwd: filepath.Join(root, "managed")},
	}
	for _, workspace := range workspaces {
		if err := os.MkdirAll(workspace.cwd, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	wrapper := writeT3CapabilityWrapper(t, root, "/bin/false", "http://127.0.0.1:1", "/frozen/node:/usr/bin:/bin", workspaces)
	body, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`COREPACK_HOME="$workspace/.capability-cache/corepack"`)) {
		t.Fatal("T3 wrapper does not isolate Corepack to the arm workspace")
	}
	for _, expected := range []string{"PATH='/frozen/node:/usr/bin:/bin'", "COREPACK_ENABLE_NETWORK=0", "COREPACK_DEFAULT_TO_LATEST=0"} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("T3 wrapper missing frozen environment %q", expected)
		}
	}
}

func TestPairT3PrepareFrontendUsesPinnedPrivateCorepack(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "pnpm.log")
	pnpm := filepath.Join(root, "pnpm")
	body := "#!/bin/sh\nset -eu\nprintf '%s|%s\\n' \"${COREPACK_HOME:-}\" \"$*\" >> " + pairShellQuote(logPath) + "\nif [ \"${1:-}\" = --version ]; then printf '10.28.2\\n'; fi\n"
	if err := os.WriteFile(pnpm, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	spec := pairT3Spec{PnpmVersion: pairT3PnpmVersion}
	spec.Toolchain.Pnpm.Path = pnpm
	corepackHome := filepath.Join(root, "private-corepack")
	if err := pairT3PrepareFrontend(spec, root, filepath.Join(root, "store"), corepackHome); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(log), corepackHome+"|") != 2 || !strings.Contains(string(log), "|install --offline --frozen-lockfile --store-dir ") {
		t.Fatalf("private Corepack environment not used for both pnpm invocations: %q", log)
	}
}

func TestPairT3WritableRootsContainImplicitAndPrivateCacheGrants(t *testing.T) {
	root := t.TempDir()
	probeTmp := filepath.Join(root, "probe-tmp")
	t.Setenv("TMPDIR", probeTmp)
	cwd, cache, goCache := filepath.Join(root, "repository"), filepath.Join(root, "repository", ".capability-cache"), filepath.Join(root, "go-cache")
	for _, directory := range []string{cwd, cache, filepath.Join(cache, "go-mod"), filepath.Join(cache, "pnpm-store"), filepath.Join(cache, "corepack"), goCache, probeTmp} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := pairT3WritableRoots(cwd, cache, goCache)
	if err != nil {
		t.Fatal(err)
	}
	expectedRoots := []string{cwd, cache, filepath.Join(cache, "go-mod"), filepath.Join(cache, "pnpm-store"), filepath.Join(cache, "corepack"), goCache, "/tmp"}
	if _, err := os.Stat("/private/tmp"); err == nil {
		expectedRoots = append(expectedRoots, "/private/tmp")
	}
	if tmpdir := os.Getenv("TMPDIR"); filepath.IsAbs(tmpdir) {
		expectedRoots = append(expectedRoots, filepath.Clean(tmpdir))
	}
	for index, expected := range expectedRoots {
		resolved, err := filepath.EvalSymlinks(expected)
		if err != nil {
			t.Fatal(err)
		}
		expectedRoots[index] = resolved
	}
	slices.Sort(expectedRoots)
	expectedRoots = slices.Compact(expectedRoots)
	if !slices.Equal(roots, expectedRoots) {
		t.Fatalf("writable roots = %q, want exact canonical roots %q", roots, expectedRoots)
	}
	implicitTmp, err := os.MkdirTemp("/tmp", "multica-pair-t3-containment-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(implicitTmp) })
	insideSentinels := []string{
		filepath.Join(implicitTmp, "sentinel"),
		filepath.Join(cwd, "sentinel"),
		filepath.Join(cache, "sentinel"),
		filepath.Join(goCache, "sentinel"),
		filepath.Join(probeTmp, "sentinel"),
	}
	for _, file := range insideSentinels {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if pairT3OutsideEveryRoot(file, roots) {
			t.Fatalf("path inside writable root reported outside: %s", file)
		}
	}
	realOutsideRoot, err := os.MkdirTemp("/var/tmp", "multica-pair-t3-outside-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(realOutsideRoot) })
	realOutsideSentinel := filepath.Join(realOutsideRoot, "sentinel")
	if err := os.WriteFile(realOutsideSentinel, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !pairT3OutsideEveryRoot(realOutsideSentinel, roots) {
		t.Fatal("path outside complete writable roots reported inside")
	}

	containmentRoot := t.TempDir()
	grantedRoot := filepath.Join(containmentRoot, "granted")
	grantedSentinel := filepath.Join(grantedRoot, "sentinel")
	outsideSentinel := filepath.Join(containmentRoot, "outside", "sentinel")
	for _, file := range []string{grantedSentinel, outsideSentinel} {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("sentinel\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	canonicalGrantedRoot, err := filepath.EvalSymlinks(grantedRoot)
	if err != nil {
		t.Fatal(err)
	}
	isolatedRoots := []string{canonicalGrantedRoot}
	if pairT3OutsideEveryRoot(grantedSentinel, isolatedRoots) {
		t.Fatal("path inside isolated writable root reported outside")
	}
	if !pairT3OutsideEveryRoot(outsideSentinel, isolatedRoots) {
		t.Fatal("sibling outside isolated writable root reported inside")
	}
}

func TestPairT3PnpmStoreRejectsProjectsRegistry(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "v10", "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := pairT3RequirePnpmStoreSeed(root); err == nil {
		t.Fatal("pnpm projects registry was accepted")
	}
}

func TestCodexT3CapabilityUsesOrdinaryDaemonMode(t *testing.T) {
	root := t.TempDir()
	cfg := pairT3DaemonConfig(root, "/bin/false", "http://127.0.0.1:1")
	if cfg.DirectAgentMode {
		t.Fatal("T3 capability branch must use ordinary daemon mode")
	}
	task := pairT3Task(root, nil)
	if task.ConciseMode {
		t.Fatal("T3 capability task must not request concise/direct behavior")
	}
	var ref localDirectoryRef
	if err := json.Unmarshal(task.ProjectResources[0].ResourceRef, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.ExecutionMode != "" {
		t.Fatalf("T3 capability task execution_mode = %q, want ordinary default", ref.ExecutionMode)
	}
}

func TestCodexT3CapabilityToolOutputBindsOneMarker(t *testing.T) {
	digest := "sha256:" + pairT3Repeat('a', 64)
	request := map[string]any{"input": []any{map[string]any{
		"type": "function_call_output", "output": "T3_CAPABILITY_PROBE_RECEIPT " + digest + "\nProcess exited with code 0",
	}}}
	ok, observed := pairT3ToolOutputProof(request)
	if !ok || observed != digest {
		t.Fatalf("valid marker proof rejected: ok=%t digest_match=%t", ok, observed == digest)
	}
	request["input"] = append(request["input"].([]any), map[string]any{"type": "function_call_output", "output": "T3_CAPABILITY_PROBE_RECEIPT " + digest})
	if ok, _ := pairT3ToolOutputProof(request); ok {
		t.Fatal("duplicate marker was accepted")
	}
}

func pairT3SHA(value string) string {
	return "sha256:" + pairT3Digest([]byte(value))
}

func pairT3SourceLocks(root string) []any {
	values := make([]any, 8)
	for index := range values {
		values[index] = map[string]any{"path": filepath.Join(root, fmt.Sprintf("source-%d.mjs", index)), "sha256": pairT3SHA(fmt.Sprintf("source-%d", index))}
	}
	return values
}

func pairT3Repeat(value byte, length int) string {
	data := make([]byte, length)
	for index := range data {
		data[index] = value
	}
	return string(data)
}

package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeV2BaseDirectory materializes base/ from INLINE artifact text. A task
// that carries a reference instead — schema, slot and digest, with no
// contents — produces an agent workspace that cannot be written, and the task
// dies before the agent ever starts.
//
// This matters because the two shapes both look like a "base package" in a
// task context, and only one of them works. The copy path builds the inline
// shape for exactly this reason.
func TestV2BaseDirectoryNeedsInlineArtifactsNotAReference(t *testing.T) {
	inline := map[string]any{
		"design_md":        "# Acme\n\n## Principles\n\nCalm.\n",
		"tokens_css":       ":root { --color-action: #1677ff; }\n",
		"components_html":  `<section data-design-node-id="b" data-design-node-kind="block" data-design-node-label="B">x</section>`,
		"integrity_sha256": strings.Repeat("a", 64),
	}
	inlineJSON, err := json.Marshal(inline)
	if err != nil {
		t.Fatalf("marshal inline base: %v", err)
	}
	workDir := t.TempDir()
	// base/ is stamped read-only, which also blocks TempDir cleanup.
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(workDir, "base"), 0o755) })
	if err := writeV2BaseDirectory(workDir, map[string]json.RawMessage{
		"operation":    json.RawMessage(`"generate"`),
		"base_package": inlineJSON,
	}, &sidecarManifest{}); err != nil {
		t.Fatalf("inline base payload must materialize, got: %v", err)
	}

	// The reference shape a V2 package decoder returns must be rejected
	// loudly rather than producing a half-written base directory.
	referenceDir := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(referenceDir, "base"), 0o755) })
	err = writeV2BaseDirectory(referenceDir, map[string]json.RawMessage{
		"operation": json.RawMessage(`"generate"`),
		"base_package": json.RawMessage(`{
			"schema": "multica.project-design-system/v2",
			"slot": "saved",
			"integrity_sha256": "sha256:aaaa"
		}`),
	}, &sidecarManifest{})
	if err == nil {
		t.Fatal("a reference-only base package was accepted; it cannot materialize base/ and the task would die writing its workspace")
	}
}

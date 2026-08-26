package execenv

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// designDocumentBaseDir is the single place the base directory's location is
// spelled out. writeDesignDocumentContext reserves it and
// ExtractDesignDocumentBase fills it; if the two ever named different paths the
// agent would read an empty base while a full package sat elsewhere on disk.
func designDocumentBaseDir(workDir string) string {
	return filepath.Join(workDir, ".agent_context", "design_document", "base")
}

// ExtractDesignDocumentBase writes a verified base revision's package into the
// directory writeDesignDocumentContext reserved, then stamps the whole tree
// read-only.
//
// The caller must have validated the archive first (see
// designdocument.ReadBaseArchive) — this function writes bytes to the agent's
// filesystem and does not re-derive trust. What it does own is the path
// contract: every entry name is checked with safeDesignDocumentBaseName before
// it becomes a filesystem path, so a name that could escape base/ fails the
// task instead of landing outside it.
//
// Read-only is stamped last and only on success. A half-written base that the
// agent could still edit is worse than no base at all: the run would silently
// adjust something other than the revision it claims to.
//
// envRoot is the daemon scratch directory holding the sidecar manifest. Every
// path created here is appended to it, because this step runs after
// execenv.Prepare persisted its own list and an unrecorded base is
// indistinguishable from user content at cleanup time — see
// appendSidecarManifest. An empty envRoot disables the bookkeeping, matching
// CleanupSidecars, which no-ops on the same condition.
func ExtractDesignDocumentBase(envRoot, workDir string, files map[string][]byte) error {
	baseDir := designDocumentBaseDir(workDir)
	if _, err := os.Stat(baseDir); err != nil {
		return fmt.Errorf("design document base directory is not reserved: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("design document base package has no files")
	}
	// Sorted so a rejected entry fails at the same point on every run; a map
	// walk would make the same bad package fail after a different number of
	// partial writes each time.
	names := make([]string, 0, len(files))
	for name := range files {
		if !safeDesignDocumentBaseName(name) {
			return fmt.Errorf("unsafe design document base entry %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	// Recorded before the stamp, so a failure between the two still leaves the
	// bytes reversible. baseDir itself is already in Prepare's manifest;
	// recordMkdirAll stops at the first existing ancestor, so only the
	// sub-directories this package introduces are added.
	extracted := &sidecarManifest{}
	for _, name := range names {
		target := filepath.Join(baseDir, filepath.FromSlash(name))
		if err := recordMkdirAll(filepath.Dir(target), 0o755, extracted); err != nil {
			return fmt.Errorf("create design document base directory for %q: %w", name, err)
		}
		if err := recordWriteFile(target, files[name], 0o444, extracted); err != nil {
			return fmt.Errorf("write design document base entry %q: %w", name, err)
		}
	}
	if err := appendSidecarManifest(envRoot, extracted); err != nil {
		return fmt.Errorf("record design document base in the sidecar manifest: %w", err)
	}
	return stampV2ReadOnly(baseDir)
}

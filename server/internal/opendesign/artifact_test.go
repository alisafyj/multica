package opendesign

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerifyWorkerArtifactMatchesLockfileAndSortedDistTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "daemon", "dist", "nested"), 0o755); err != nil {
		t.Fatalf("create dist: %v", err)
	}
	files := map[string]string{
		"pnpm-lock.yaml":                    "lock",
		"apps/daemon/dist/a.js":             "alpha",
		"apps/daemon/dist/nested/worker.js": "beta",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	lockDigest := sha256.Sum256([]byte("lock"))
	treeDigest := sha256.Sum256([]byte("a.js\x00alpha\x00nested/worker.js\x00beta\x00"))
	expected := PinnedEngineIdentity()
	expected.LockfileSHA256 = hex.EncodeToString(lockDigest[:])
	expected.DistSHA256 = hex.EncodeToString(treeDigest[:])

	verification, err := VerifyWorkerArtifact(root, expected)
	if err != nil {
		t.Fatalf("VerifyWorkerArtifact: %v", err)
	}
	if verification.FileCount != 2 || verification.ByteCount != int64(len("alpha")+len("beta")) {
		t.Fatalf("verification counts = %+v", verification)
	}
	if verification.LockfileSHA256 != expected.LockfileSHA256 || verification.DistSHA256 != expected.DistSHA256 {
		t.Fatalf("verification digests = %+v", verification)
	}
}

func TestVerifyWorkerArtifactRejectsDigestMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "daemon", "dist"), 0o755); err != nil {
		t.Fatalf("create dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("wrong"), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "daemon", "dist", "cli.js"), []byte("worker"), 0o600); err != nil {
		t.Fatalf("write worker: %v", err)
	}

	_, err := VerifyWorkerArtifact(root, PinnedEngineIdentity())
	if err == nil || !strings.Contains(err.Error(), "lockfile_sha256") {
		t.Fatalf("VerifyWorkerArtifact error = %v, want lockfile_sha256 mismatch", err)
	}
}

func TestVerifyWorkerArtifactRejectsSymlinkInDist(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "apps", "daemon", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("create dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
	target := filepath.Join(root, "outside.js")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(dist, "worker.js")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	expected := PinnedEngineIdentity()
	lockDigest := sha256.Sum256([]byte("lock"))
	expected.LockfileSHA256 = hex.EncodeToString(lockDigest[:])
	_, err := VerifyWorkerArtifact(root, expected)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("VerifyWorkerArtifact error = %v, want symlink rejection", err)
	}
}

func TestVerifyWorkerArtifactRejectsNonRegularFileInDist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix domain socket fixture is not available on Windows")
	}
	root, err := os.MkdirTemp("/tmp", "od-artifact-")
	if err != nil {
		t.Fatalf("create short artifact root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	dist := filepath.Join(root, "apps", "daemon", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("create dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
	listener, err := net.Listen("unix", filepath.Join(dist, "worker.sock"))
	if err != nil {
		t.Fatalf("create Unix socket: %v", err)
	}
	defer listener.Close()

	expected := PinnedEngineIdentity()
	lockDigest := sha256.Sum256([]byte("lock"))
	expected.LockfileSHA256 = hex.EncodeToString(lockDigest[:])
	_, err = VerifyWorkerArtifact(root, expected)
	if err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("VerifyWorkerArtifact error = %v, want non-regular file rejection", err)
	}
}

func TestVerifyPinnedWorkerArtifactCheckout(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("OPEN_DESIGN_PINNED_CHECKOUT"))
	if root == "" {
		t.Skip("OPEN_DESIGN_PINNED_CHECKOUT is not set")
	}
	verification, err := VerifyWorkerArtifact(root, PinnedEngineIdentity())
	if err != nil {
		t.Fatalf("VerifyWorkerArtifact(%q): %v", root, err)
	}
	t.Logf(
		"verified pinned worker artifact: files=%d bytes=%d lockfile_sha256=%s dist_sha256=%s",
		verification.FileCount,
		verification.ByteCount,
		verification.LockfileSHA256,
		verification.DistSHA256,
	)
}

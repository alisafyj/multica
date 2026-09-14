//go:build darwin || linux

package agent

import (
	"os"
	"syscall"
)

func codexPluginCacheFileIdentity(info os.FileInfo) (uint64, uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(stat.Dev), uint64(stat.Ino), true
}

func codexPluginMetadataOwnedDirectory(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.IsDir() && info.Mode().Perm() == 0o700 && stat.Uid == uint32(os.Geteuid())
}

//go:build !darwin && !linux

package agent

import "os"

func codexPluginCacheFileIdentity(os.FileInfo) (uint64, uint64, bool) {
	return 0, 0, false
}

func codexPluginMetadataOwnedDirectory(os.FileInfo) bool {
	return false
}

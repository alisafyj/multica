package handler

import "testing"

func TestFigmaDesignAssetStoragePrefersDedicatedStorage(t *testing.T) {
	defaultStore := &mockStorage{}
	dedicatedStore := &mockStorage{}
	h := &Handler{Storage: defaultStore, DesignAssetStorage: dedicatedStore}

	if got := h.figmaDesignAssetStorage(); got != dedicatedStore {
		t.Fatal("figmaDesignAssetStorage() did not prefer dedicated storage")
	}
}

func TestFigmaDesignAssetStorageFallsBackToDefaultStorage(t *testing.T) {
	defaultStore := &mockStorage{}
	h := &Handler{Storage: defaultStore}

	if got := h.figmaDesignAssetStorage(); got != defaultStore {
		t.Fatal("figmaDesignAssetStorage() did not fall back to default storage")
	}
}

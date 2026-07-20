package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadFigmaDesignAssetReturnsDedicatedStorageURL(t *testing.T) {
	token := createPluginTokenForTest(t)
	dedicatedStore := &mockStorage{}
	originalStore := testHandler.DesignAssetStorage
	testHandler.DesignAssetStorage = dedicatedStore
	t.Cleanup(func() { testHandler.DesignAssetStorage = originalStore })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "figma-image.png")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\nfigma-image")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.WriteField("kind", "image"); err != nil {
		t.Fatalf("WriteField() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/design-plugin/figma/assets", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	testHandler.UploadFigmaDesignAssetWithPluginToken(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UploadFigmaDesignAssetWithPluginToken() status = %d, body = %s", w.Code, w.Body.String())
	}

	var response UploadFigmaDesignAssetResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !strings.HasPrefix(response.URL, "https://cdn.example.com/workspaces/"+testWorkspaceID+"/design-assets/") {
		t.Fatalf("response URL = %q, want dedicated CDN URL", response.URL)
	}
	if response.Kind != "image" {
		t.Fatalf("response kind = %q, want image", response.Kind)
	}
}

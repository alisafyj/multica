package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type recordingQiniuUploader struct {
	request qiniuUploadRequest
	err     error
}

func (u *recordingQiniuUploader) Upload(_ context.Context, request qiniuUploadRequest) error {
	u.request = request
	return u.err
}

type recordingQiniuDeleter struct {
	bucket string
	key    string
	err    error
}

func (d *recordingQiniuDeleter) Delete(_ context.Context, bucket, key string) error {
	d.bucket = bucket
	d.key = key
	return d.err
}

func TestQiniuStorageUploadReturnsCDNURL(t *testing.T) {
	uploader := &recordingQiniuUploader{}
	store := newQiniuStorage(qiniuConfig{
		AccessKey:  "test-ak",
		SecretKey:  "test-sk",
		Bucket:     "static",
		CDNBaseURL: "https://static.soyoung.com",
		KeyPrefix:  "sy-design",
	}, uploader, &recordingQiniuDeleter{}, http.DefaultClient)

	data := []byte("figma-image")
	got, err := store.Upload(context.Background(), "workspaces/ws/design-assets/image.png", data, "image/png", "image.png")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if want := "https://static.soyoung.com/sy-design/workspaces/ws/design-assets/image.png"; got != want {
		t.Fatalf("Upload() URL = %q, want %q", got, want)
	}
	if uploader.request.UploadToken == "" {
		t.Fatal("Upload() did not create a Qiniu upload token")
	}
	if want := "sy-design/workspaces/ws/design-assets/image.png"; uploader.request.Key != want {
		t.Fatalf("uploaded key = %q, want %q", uploader.request.Key, want)
	}
	if !bytes.Equal(uploader.request.Data, data) {
		t.Fatalf("uploaded data = %q, want %q", uploader.request.Data, data)
	}
	if uploader.request.ContentType != "image/png" {
		t.Fatalf("uploaded content type = %q, want image/png", uploader.request.ContentType)
	}
}

func TestQiniuStorageKeyFromURLAndDelete(t *testing.T) {
	deleter := &recordingQiniuDeleter{}
	store := newQiniuStorage(qiniuConfig{
		AccessKey:  "test-ak",
		SecretKey:  "test-sk",
		Bucket:     "static",
		CDNBaseURL: "https://static.soyoung.com/",
		KeyPrefix:  "sy-design/",
	}, &recordingQiniuUploader{}, deleter, http.DefaultClient)

	rawURL := "https://static.soyoung.com/sy-design/workspaces/ws/design-assets/image.png?imageView2/0/w/600"
	key := store.KeyFromURL(rawURL)
	if want := "sy-design/workspaces/ws/design-assets/image.png"; key != want {
		t.Fatalf("KeyFromURL() = %q, want %q", key, want)
	}

	store.Delete(context.Background(), key)
	if deleter.bucket != "static" || deleter.key != key {
		t.Fatalf("Delete() = bucket %q key %q, want static/%s", deleter.bucket, deleter.key, key)
	}
	if got := store.CdnDomain(); got != "static.soyoung.com" {
		t.Fatalf("CdnDomain() = %q, want static.soyoung.com", got)
	}
}

func TestQiniuStorageGetReaderUsesCDN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/sy-design/workspaces/ws/design-assets/image.png"; r.URL.Path != want {
			t.Fatalf("request path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("cdn-image"))
	}))
	defer server.Close()

	store := newQiniuStorage(qiniuConfig{
		AccessKey:  "test-ak",
		SecretKey:  "test-sk",
		Bucket:     "static",
		CDNBaseURL: server.URL,
		KeyPrefix:  "sy-design",
	}, &recordingQiniuUploader{}, &recordingQiniuDeleter{}, server.Client())

	reader, err := store.GetReader(context.Background(), "workspaces/ws/design-assets/image.png")
	if err != nil {
		t.Fatalf("GetReader() error = %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if want := "cdn-image"; string(got) != want {
		t.Fatalf("GetReader() body = %q, want %q", got, want)
	}
}

func TestNewQiniuStorageFromEnv(t *testing.T) {
	t.Setenv("QINIU_ACCESS_KEY", "")
	t.Setenv("QINIU_SECRET_KEY", "")
	if got := NewQiniuStorageFromEnv(); got != nil {
		t.Fatal("NewQiniuStorageFromEnv() should be nil without credentials")
	}

	t.Setenv("QINIU_ACCESS_KEY", "test-ak")
	t.Setenv("QINIU_SECRET_KEY", "test-sk")
	t.Setenv("QINIU_BUCKET", "")
	t.Setenv("QINIU_CDN_BASE_URL", "")
	t.Setenv("QINIU_KEY_PREFIX", "")
	store := NewQiniuStorageFromEnv()
	if store == nil {
		t.Fatal("NewQiniuStorageFromEnv() returned nil with credentials")
	}
	if store.bucket != "static" {
		t.Fatalf("bucket = %q, want static", store.bucket)
	}
	if store.cdnBaseURL != "https://static.soyoung.com" {
		t.Fatalf("cdnBaseURL = %q, want https://static.soyoung.com", store.cdnBaseURL)
	}
	if store.keyPrefix != "sy-design" {
		t.Fatalf("keyPrefix = %q, want sy-design", store.keyPrefix)
	}
}

func TestQiniuStorageLiveUpload(t *testing.T) {
	if os.Getenv("QINIU_LIVE_TEST") != "1" {
		t.Skip("set QINIU_LIVE_TEST=1 to run against Qiniu")
	}
	store := NewQiniuStorageFromEnv()
	if store == nil {
		t.Fatal("Qiniu credentials are required for the live test")
	}

	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	key := "codex-verification/figma-asset-" + time.Now().UTC().Format("20060102T150405.000000000") + ".png"
	url, err := store.Upload(context.Background(), key, png, "image/png", "figma-verification.png")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	t.Cleanup(func() { store.Delete(context.Background(), key) })
	if !strings.HasPrefix(url, "https://static.soyoung.com/sy-design/") {
		t.Fatalf("Upload() URL = %q, want static.soyoung.com/sy-design prefix", url)
	}

	var reader io.ReadCloser
	for attempt := 0; attempt < 10; attempt++ {
		reader, err = store.GetReader(context.Background(), key)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GetReader() error after CDN retries = %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("CDN body length = %d, want %d", len(got), len(png))
	}
}

//go:build vim4

package download

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCancelledAndChecksumFailedDownloadsNeverPublishImages(t *testing.T) {
	old := imageDirectory
	imageDirectory = t.TempDir()
	defer func() { imageDirectory = old }()
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad.iso" {
			_, _ = w.Write([]byte("bad checksum"))
			return
		}
		_, _ = w.Write([]byte("partial image"))
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	service := &Service{}
	sum := sha256.Sum256([]byte("expected image"))
	if err := service.downloadRemoteImage(context.Background(), server.URL+"/bad.iso", sum[:], "bad.iso", nil); !errors.Is(err, errSHA256Mismatch) {
		t.Fatalf("checksum error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.downloadRemoteImage(ctx, server.URL+"/slow.iso", nil, "slow.iso", nil) }()
	<-ready
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	files, err := os.ReadDir(imageDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("unfinished images remain: %v", files)
	}
}

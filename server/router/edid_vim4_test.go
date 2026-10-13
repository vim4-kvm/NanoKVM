//go:build vim4

package router

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func testEdid() []byte {
	data := append([]byte{0, 255, 255, 255, 255, 255, 255, 0}, make([]byte, 120)...)
	var sum byte
	for _, b := range data[:127] {
		sum += b
	}
	data[127] = -sum
	return data
}

func uploadEdid(t *testing.T, name string, data []byte) string {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", name)
	_, _ = part.Write(data)
	_ = form.Close()
	r := gin.New()
	r.POST("/upload", uploadEdidVIM4)
	w := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/upload", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	r.ServeHTTP(w, request)
	return w.Body.String()
}

func TestEdidUploadRejectsUnsafeOrInvalidFiles(t *testing.T) {
	customEdidDir = t.TempDir()
	broken := testEdid()
	broken[20] ^= 1
	for _, c := range []struct {
		name string
		data []byte
	}{
		{".hidden.bin", testEdid()}, {"edid.txt", testEdid()},
		{"broken.bin", broken}, {"short.bin", testEdid()[:100]}, {"long.bin", append(testEdid(), testEdid()...)},
	} {
		if strings.Contains(uploadEdid(t, c.name, c.data), `"code":0`) {
			t.Errorf("accepted %s", c.name)
		}
	}
	if entries, _ := os.ReadDir(customEdidDir); len(entries) != 0 {
		t.Fatalf("rejected uploads left files: %v", entries)
	}
	// multipart reduces client paths to their base name.
	uploadEdid(t, "../escape.bin", testEdid())
	if _, err := os.Stat(filepath.Join(filepath.Dir(customEdidDir), "escape.bin")); err == nil {
		t.Fatal("upload escaped the EDID directory")
	}
}

func TestEdidUploadIsListedAndReadable(t *testing.T) {
	customEdidDir = t.TempDir()
	if body := uploadEdid(t, "Monitor_1080p.bin", testEdid()); !strings.Contains(body, `"code":0`) {
		t.Fatalf("valid upload rejected: %s", body)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(customEdidDir, "link.bin")); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/list", customEdidListVIM4)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/list", nil))
	if !strings.Contains(w.Body.String(), `"edidList":["Monitor_1080p.bin"]`) {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
	if data, err := readCustomEdid("Monitor_1080p.bin"); err != nil || !bytes.Equal(data, testEdid()) {
		t.Fatalf("stored EDID unreadable: %v", err)
	}
}

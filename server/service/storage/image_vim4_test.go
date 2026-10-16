//go:build vim4

package storage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMediaOperationsRefuseExternalAndOccupiedDisks(t *testing.T) {
	previousRead, previousOwned := mountedImage, ownedImage
	t.Cleanup(func() { mountedImage, ownedImage = previousRead, previousOwned })
	current := USBDiskImage
	mountedImage = func() string { return current }
	ownedImage = ""
	if _, err := ToggleDataDisk(context.Background()); err == nil {
		t.Fatal("legacy switch accepted externally mounted disk")
	}
	for _, args := range [][]string{{"eject"}, {"mount", "data/work.img", "false", "false"}} {
		if err := mediaOperation(context.Background(), args...); err == nil {
			t.Fatalf("accepted external media operation %v", args)
		}
	}
	ownedImage = USBDiskImage
	current = "/var/lib/vim4-kvm/usb/work.img"
	if _, err := ToggleDataDisk(context.Background()); err == nil || ownedImage != "" {
		t.Fatal("legacy switch replaced another disk or retained stale ownership")
	}
}

func TestImageResolutionRejectsHostPathsAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "disk.img"), []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveImagePath(dir, "disk.img"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "disk.img"), filepath.Join(dir, "link.iso")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../disk.img", "/dev/mmcblk0", "link.iso", "/etc/passwd"} {
		if _, err := resolveImagePath(dir, name); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestCatalogListRetainsFilesAPIAndProtectsForeignDeletion(t *testing.T) {
	previous := readCatalog
	t.Cleanup(func() { readCatalog = previous })
	readCatalog = func(context.Context) ([]catalogImage, error) {
		return []catalogImage{
			{ID: "shared/boot.iso", Name: "boot.iso", Source: "shared", CDROM: true},
			{ID: "jetkvm/boot.iso", Name: "boot.iso", Source: "jetkvm", CDROM: true},
			{ID: "data/work.img", Name: "work.img", Source: "data", Disk: true},
		}, nil
	}
	router := gin.New()
	service := NewService()
	router.GET("/images", service.GetImages)
	router.POST("/delete", service.DeleteImage)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/images", nil))
	var response struct {
		Code int `json:"code"`
		Data struct {
			Files   []string       `json:"files"`
			Catalog []catalogImage `json:"catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || len(response.Data.Files) != 3 || len(response.Data.Catalog) != 3 {
		t.Fatalf("catalog response: %s", w.Body.String())
	}
	if response.Data.Files[0] != ImageDirectory+"/boot.iso" || response.Data.Files[1] != "jetkvm/boot.iso" {
		t.Fatalf("ambiguous filenames: %v", response.Data.Files)
	}
	for _, id := range response.Data.Files[1:] {
		w := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/delete", strings.NewReader(`{"file":"`+id+`"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, request)
		var result struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Code == 0 {
			t.Fatalf("foreign delete accepted: %s", id)
		}
	}
}

//go:build vim4

package storage

import (
	"os"
	"path/filepath"
	"testing"
)

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

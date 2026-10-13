//go:build vim4

package application

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeBundle(t *testing.T, dir string, headers []*tar.Header, bodies [][]byte) string {
	t.Helper()
	path := filepath.Join(dir, "bundle.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for i, header := range headers {
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(bodies[i]) > 0 {
			if _, err := tw.Write(bodies[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBundleRejectsTraversalLinksFirmwareAndDuplicateMembers(t *testing.T) {
	for _, header := range []*tar.Header{
		{Name: "../application.deb", Typeflag: tar.TypeReg, Size: 1},
		{Name: "nanokvm_pro_1.2.3/../application.deb", Typeflag: tar.TypeReg, Size: 1},
		{Name: "nanokvm_pro_1.2.3/vim4-nanokvm_1.2.3_arm64.deb", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		{Name: "nanokvm_pro_1.2.3/firmware.img", Typeflag: tar.TypeReg, Size: 1},
	} {
		t.Run(header.Name, func(t *testing.T) {
			dir := t.TempDir()
			data := []byte{}
			if header.Size > 0 {
				data = []byte{1}
			}
			archive := writeBundle(t, dir, []*tar.Header{header}, [][]byte{data})
			if _, err := extractVIM4Bundle(archive, dir, ""); err == nil {
				t.Fatal("unsafe bundle accepted")
			}
		})
	}
	dir := t.TempDir()
	h := &tar.Header{Name: "nanokvm_pro_1.2.3/", Typeflag: tar.TypeDir}
	archive := writeBundle(t, dir, []*tar.Header{h, h}, [][]byte{nil, nil})
	if _, err := extractVIM4Bundle(archive, dir, ""); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestBundleValidatesRealDebMetadataAndChecksum(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Skip("dpkg-deb required")
	}
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package")
	if err := os.MkdirAll(filepath.Join(pkg, "DEBIAN"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(pkg, "DEBIAN"), 0755); err != nil {
		t.Fatal(err)
	}
	control := "Package: vim4-nanokvm\nVersion: 1.2.3\nArchitecture: arm64\nMaintainer: Test <test@example.invalid>\nDescription: test fixture\n"
	if err := os.WriteFile(filepath.Join(pkg, "DEBIAN/control"), []byte(control), 0600); err != nil {
		t.Fatal(err)
	}
	deb := filepath.Join(dir, "fixture.deb")
	if output, err := exec.Command("dpkg-deb", "--build", pkg, deb).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	data, err := os.ReadFile(deb)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha512.Sum512(data)
	name := "vim4-nanokvm_1.2.3_arm64.deb"
	root := "nanokvm_pro_1.2.3/"
	metadata, _ := json.Marshal(map[string]any{"version": "1.2.3", "name": name, "size": len(data), "sha512": base64.StdEncoding.EncodeToString(hash[:])})
	headers := []*tar.Header{{Name: root, Typeflag: tar.TypeDir}, {Name: root + name, Typeflag: tar.TypeReg, Size: int64(len(data))}, {Name: root + "vim4-nanokvm_1.2.3.json", Typeflag: tar.TypeReg, Size: int64(len(metadata))}}
	archive := writeBundle(t, dir, headers, [][]byte{nil, data, metadata})
	if _, err := extractVIM4Bundle(archive, t.TempDir(), "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if _, err := extractVIM4Bundle(archive, t.TempDir(), "1.2.4"); err == nil {
		t.Fatal("version mismatch accepted")
	}
	data[len(data)-1] ^= 1
	archive = writeBundle(t, dir, headers, [][]byte{nil, data, metadata})
	if _, err := extractVIM4Bundle(archive, t.TempDir(), "1.2.3"); err == nil {
		t.Fatal("corrupt deb accepted")
	}
}

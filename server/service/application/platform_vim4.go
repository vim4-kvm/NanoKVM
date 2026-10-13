//go:build vim4

package application

import (
	"NanoKVM-Server/proto"
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	StableURL          = ""
	PreviewURL         = ""
	AppDir             = "/opt/vim4-nanokvm"
	BackupDir          = "/var/lib/vim4-kvm/update-cache/unused-backup"
	CacheDir           = "/var/lib/vim4-kvm/update-cache"
	latestManifestName = "nanokvm_pro_latest.json"
	packagePattern     = `^nanokvm_pro_[0-9]+\.[0-9]+\.[0-9]+(?:[+.-][A-Za-z0-9.-]+)?\.tar\.gz$`
)

func platformUpdate(c *gin.Context, offline bool) bool {
	var rsp proto.Response
	if !acquireUpdateLock() {
		rsp.ErrRsp(c, -1, "update already in progress")
		return true
	}
	// On success the helper schedules a restart; retain the lock until exit.
	err := installVIM4Update(c, offline)
	if err != nil {
		releaseUpdateLock()
		rsp.ErrRsp(c, -1, err.Error())
		return true
	}
	rsp.OkRsp(c)
	return true
}

func installVIM4Update(c *gin.Context, offline bool) error {
	if err := os.MkdirAll(CacheDir, 0o700); err != nil {
		return err
	}
	workspace, err := os.MkdirTemp(CacheDir, "bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	archive := filepath.Join(workspace, "bundle.tar.gz")
	expectedVersion := ""
	if offline {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxPackageSize)+(1<<20))
		reader, err := c.Request.MultipartReader()
		if err != nil {
			return errors.New("invalid upload")
		}
		found := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if part.FormName() != "file" {
				_ = part.Close()
				continue
			}
			if found {
				return errors.New("expected one application bundle")
			}
			found = true
			if !packageNamePattern.MatchString(part.FileName()) {
				return errors.New("expected a VIM4 application bundle")
			}
			out, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			hash := sha256.New()
			n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(part, int64(maxPackageSize)+1))
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n > int64(maxPackageSize) {
				return errors.New("bundle too large")
			}
			if expected := c.GetHeader("X-SHA256-Checksum"); expected != "" {
				sum, err := parseSHA256Checksum(expected)
				if err != nil {
					return err
				}
				if hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(sum) {
					return errors.New("SHA256 mismatch")
				}
			}
		}
		if !found {
			return errors.New("application bundle is required")
		}
	} else {
		latest, err := getLatest()
		if err != nil {
			return err
		}
		expectedVersion = latest.Version
		if _, err = download(latest, archive); err != nil {
			return err
		}
		if err = checksum(archive, latest.Sha512); err != nil {
			return err
		}
	}
	deb, err := extractVIM4Bundle(archive, workspace, expectedVersion)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-update", deb).Run(); err != nil {
		return errors.New("Ubuntu package installation failed; see service log")
	}
	return nil
}

// Accept only the two files emitted by deploy/vim4/package.py. No archive path,
// symlink, firmware payload or executable script is extracted from the wrapper.
func extractVIM4Bundle(archive, workspace, expectedVersion string) (string, error) {
	file, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	seen := map[string]bool{}
	var root, version, deb string
	var manifest struct {
		Version string `json:"version"`
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		SHA512  string `json:"sha512"`
	}
	for count := 0; ; count++ {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if count > 3 {
			return "", errors.New("unexpected bundle entries")
		}
		name := strings.TrimSuffix(header.Name, "/")
		if seen[name] {
			return "", errors.New("duplicate bundle member")
		}
		seen[name] = true
		parts := strings.Split(name, "/")
		if root == "" {
			root = parts[0]
			if !strings.HasPrefix(root, "nanokvm_pro_") {
				return "", errors.New("invalid bundle root")
			}
			version = strings.TrimPrefix(root, "nanokvm_pro_")
			if !versionPattern.MatchString(version) || (expectedVersion != "" && version != expectedVersion) {
				return "", errors.New("invalid bundle version")
			}
		}
		if parts[0] != root {
			return "", errors.New("mixed bundle roots")
		}
		if header.Typeflag == tar.TypeDir && len(parts) == 1 {
			continue
		}
		if !isRegularType(header.Typeflag) || len(parts) != 2 || header.Size <= 0 || header.Size > 512<<20 {
			return "", errors.New("invalid bundle member")
		}
		debName := "vim4-nanokvm_" + version + "_arm64.deb"
		switch parts[1] {
		case debName:
			if err := ensureFreeSpace(workspace, uint64(header.Size)); err != nil {
				return "", err
			}
			deb = filepath.Join(workspace, debName)
			out, err := os.OpenFile(deb, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return "", err
			}
			_, err = io.CopyN(out, reader, header.Size)
			closeErr := out.Close()
			if err != nil {
				return "", err
			}
			if closeErr != nil {
				return "", closeErr
			}
		case "vim4-nanokvm_" + version + ".json":
			if header.Size > 65536 {
				return "", errors.New("metadata too large")
			}
			if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
				return "", err
			}
		default:
			return "", errors.New("unexpected bundle member")
		}
	}
	if deb == "" || manifest.Name != filepath.Base(deb) || manifest.Version != version {
		return "", errors.New("bundle metadata mismatch")
	}
	info, err := os.Stat(deb)
	if err != nil {
		return "", err
	}
	if info.Size() != manifest.Size {
		return "", errors.New("bundle size mismatch")
	}
	if err := checksum(deb, manifest.SHA512); err != nil {
		return "", err
	}
	for field, expected := range map[string]string{"Package": "vim4-nanokvm", "Architecture": "arm64", "Version": version} {
		output, err := exec.Command("dpkg-deb", "-f", deb, field).Output()
		if err != nil || strings.TrimSpace(string(output)) != expected {
			return "", fmt.Errorf("invalid Debian %s", field)
		}
	}
	return deb, nil
}

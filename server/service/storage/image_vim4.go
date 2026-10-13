//go:build vim4

package storage

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ImageDirectory = "/var/lib/vim4-kvm/images"
const USBDiskImage = "/var/lib/vim4-kvm/usb/disk.img"
const mountDevice = "/sys/kernel/config/usb_gadget/kvm/functions/mass_storage.disk0/lun.0/file"

var imageMutex sync.Mutex

func imagePath(name string) (string, error) {
	return resolveImagePath(ImageDirectory, name)
}

func resolveImagePath(directory, name string) (string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(directory, name)
	}
	name = filepath.Clean(name)
	if filepath.Dir(name) != directory {
		return "", errors.New("invalid image path")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".iso" && ext != ".img" {
		return "", errors.New("invalid image extension")
	}
	info, err := os.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("image must be a regular file")
	}
	return name, nil
}
func mountedImage() string { b, _ := os.ReadFile(mountDevice); return strings.TrimSpace(string(b)) }

// PublishImage serializes publication with mounting/deletion and never replaces
// an existing image inode that the host may still have open.
func PublishImage(temp, target string) error {
	imageMutex.Lock()
	defer imageMutex.Unlock()
	if filepath.Dir(target) != ImageDirectory {
		return errors.New("invalid image destination")
	}
	if err := os.Link(temp, target); err != nil {
		return err
	}
	return os.Remove(temp)
}
func (s *Service) GetImages(c *gin.Context) {
	var rsp proto.Response
	files := []string{}
	entries, err := os.ReadDir(ImageDirectory)
	if err != nil && !os.IsNotExist(err) {
		rsp.ErrRsp(c, -1, "cannot list images")
		return
	}
	for _, entry := range entries {
		if path, err := imagePath(entry.Name()); err == nil {
			files = append(files, path)
		}
	}
	rsp.OkRspWithData(c, proto.GetImagesRsp{Files: files})
}
func (s *Service) GetMountedImage(c *gin.Context) {
	var rsp proto.Response
	file := mountedImage()
	if file == USBDiskImage {
		file = ""
	}
	rsp.OkRspWithData(c, proto.GetMountedImageRsp{File: file})
}
func (s *Service) MountImage(c *gin.Context) {
	var req proto.MountImageReq
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	imageMutex.Lock()
	defer imageMutex.Unlock()
	args := []string{"-n", "/usr/libexec/vim4-media", "eject"}
	if req.File != "" {
		path, err := imagePath(req.File)
		if err != nil {
			rsp.ErrRsp(c, -1, "invalid image")
			return
		}
		readOnly := req.ReadOnly == nil || *req.ReadOnly || req.Cdrom
		args = []string{"-n", "/usr/libexec/vim4-media", "mount", path, strconv.FormatBool(req.Cdrom), strconv.FormatBool(readOnly)}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := hid.GetHid().Reenumerate(exec.CommandContext(ctx, "sudo", args...).Run); err != nil {
		rsp.ErrRsp(c, -2, "virtual media operation failed")
		return
	}
	rsp.OkRsp(c)
}
func (s *Service) GetCdRom(c *gin.Context) {
	var rsp proto.Response
	data, err := os.ReadFile(filepath.Join(filepath.Dir(mountDevice), "cdrom"))
	if err != nil {
		rsp.ErrRsp(c, -1, "media is not configured")
		return
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid media state")
		return
	}
	rsp.OkRspWithData(c, proto.GetCdRomRsp{Cdrom: value})
}
func (s *Service) DeleteImage(c *gin.Context) {
	var req proto.DeleteImageReq
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	imageMutex.Lock()
	defer imageMutex.Unlock()
	path, err := imagePath(req.File)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid image")
		return
	}
	if path == mountedImage() {
		rsp.ErrRsp(c, -2, "image is mounted")
		return
	}
	if err := os.Remove(path); err != nil {
		rsp.ErrRsp(c, -2, "cannot delete image")
		return
	}
	rsp.OkRsp(c)
}
func SetDataWritable(bool) error { return nil }
func DataDiskAttached() bool     { return mountedImage() == USBDiskImage }

//go:build vim4

package storage

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"context"
	"encoding/json"
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
var ownedImage string

// mediaOperation must run with imageMutex held. Only this process's successful
// mount authorizes eject; the helper rechecks the expected path under board locks.
func mediaOperation(ctx context.Context, args ...string) error {
	current := mountedImage()
	if current != ownedImage {
		ownedImage = ""
	}
	eject := args[0] == "eject" || (args[0] == "disk" && args[1] == "off")
	if eject {
		if current == "" {
			return nil
		}
		if ownedImage == "" {
			return errors.New("external media must be ejected in the management panel")
		}
		args = append(args, current)
	} else if current != "" {
		return errors.New("eject existing media first")
	}
	command := append([]string{"-n", "/usr/libexec/vim4-media"}, args...)
	var output []byte
	if err := hid.GetHid().Reenumerate(func() error {
		var err error
		output, err = exec.CommandContext(ctx, "sudo", command...).Output()
		return err
	}); err != nil {
		return err
	}
	ownedImage = ""
	var result struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return err
	}
	ownedImage = result.File
	return nil
}

func ToggleDataDisk(ctx context.Context) (bool, error) {
	imageMutex.Lock()
	defer imageMutex.Unlock()
	state := "on"
	if DataDiskAttached() {
		state = "off"
	}
	err := mediaOperation(ctx, "disk", state)
	return state == "on", err
}

type catalogImage struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Size   int64  `json:"size"`
	CDROM  bool   `json:"cdrom"`
	Disk   bool   `json:"disk,omitempty"`
}

var readCatalog = func(ctx context.Context) ([]catalogImage, error) {
	data, err := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-media", "catalog").Output()
	if err != nil {
		return nil, err
	}
	var entries []catalogImage
	err = json.Unmarshal(data, &entries)
	return entries, err
}

func catalogFile(entry catalogImage) string {
	if entry.Source == "shared" {
		return filepath.Join(ImageDirectory, strings.TrimPrefix(entry.ID, "shared/"))
	}
	return entry.ID
}

func catalogID(file string) string {
	if file != "" && !strings.Contains(file, "/") {
		return "shared/" + file
	}
	roots := map[string]string{
		"shared": ImageDirectory, "data": "/var/lib/vim4-kvm/usb",
		"jetkvm": "/var/lib/jetkvm/images", "onekvm": "/var/lib/one-kvm/msd", "pikvm": "/var/lib/kvmd/msd",
	}
	for source, root := range roots {
		if strings.HasPrefix(file, root+"/") {
			return source + "/" + strings.TrimPrefix(file, root+"/")
		}
	}
	return file
}

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

var mountedImage = func() string { b, _ := os.ReadFile(mountDevice); return strings.TrimSpace(string(b)) }

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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	entries, err := readCatalog(ctx)
	if err != nil {
		rsp.ErrRsp(c, -1, "cannot list images")
		return
	}
	for _, entry := range entries {
		files = append(files, catalogFile(entry))
	}
	rsp.OkRspWithData(c, gin.H{"files": files, "catalog": entries})
}
func (s *Service) GetMountedImage(c *gin.Context) {
	imageMutex.Lock()
	defer imageMutex.Unlock()
	var rsp proto.Response
	file := mountedImage()
	if file != ownedImage {
		ownedImage = ""
	}
	if file != "" && !strings.HasPrefix(file, ImageDirectory+"/") {
		file = catalogID(file)
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
	args := []string{"eject"}
	if req.File != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		entries, err := readCatalog(ctx)
		var selected *catalogImage
		for i := range entries {
			if entries[i].ID == catalogID(req.File) {
				selected = &entries[i]
				break
			}
		}
		if err != nil || selected == nil {
			rsp.ErrRsp(c, -1, "invalid image")
			return
		}
		cdrom := (req.Cdrom || (selected.Source != "shared" && selected.CDROM)) && !selected.Disk
		readOnly := req.ReadOnly == nil || *req.ReadOnly || cdrom || (selected.Source != "shared" && selected.Source != "data")
		args = []string{"mount", selected.ID, strconv.FormatBool(cdrom), strconv.FormatBool(readOnly)}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := mediaOperation(ctx, args...); err != nil {
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

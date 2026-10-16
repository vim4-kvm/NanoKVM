//go:build vim4

package router

import (
	"NanoKVM-Server/proto"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Uploaded EDIDs stay in the application's configuration directory; only the
// root helper may load one into the HDMI receiver.
var (
	customEdidDir = "/etc/kvm/edid"
	edidPresets   = map[string]bool{"VIM4-4K60": true, "VIM4-1080P60": true, "RTK-CONTROL": true}
	edidName      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,59}\.[Bb][Ii][Nn]$`)
	edidHeader    = []byte{0, 255, 255, 255, 255, 255, 255, 0}
)

// validEdid mirrors the helper so unusable files are never listed.
func validEdid(data []byte) bool {
	if (len(data) != 128 && len(data) != 256) || !bytes.Equal(data[:8], edidHeader) || int(data[126]) != len(data)/128-1 {
		return false
	}
	for offset := 0; offset < len(data); offset += 128 {
		var sum byte
		for _, b := range data[offset : offset+128] {
			sum += b
		}
		if sum != 0 {
			return false
		}
	}
	return true
}

func edidHelper(ctx context.Context, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"-n", "/usr/libexec/vim4-edid"}, args...)...)
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func readCustomEdid(name string) ([]byte, error) {
	if !edidName.MatchString(name) {
		return nil, errors.New("invalid EDID name")
	}
	file, err := os.OpenFile(filepath.Join(customEdidDir, name), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("EDID is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 257))
	if err != nil || !validEdid(data) {
		return nil, errors.New("invalid EDID")
	}
	return data, nil
}

func getEdidVIM4(c *gin.Context) {
	var rsp proto.Response
	name, err := edidHelper(c.Request.Context(), nil, "status")
	if err != nil {
		rsp.ErrRsp(c, -1, "get edid failed")
		return
	}
	rsp.OkRspWithData(c, proto.GetEdidRsp{Edid: name})
}

func switchEdidVIM4(c *gin.Context) {
	var req proto.SwitchEdidReq
	var rsp proto.Response
	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	var err error
	if edidPresets[req.Edid] {
		_, err = edidHelper(c.Request.Context(), nil, "preset", req.Edid)
	} else {
		var data []byte
		if data, err = readCustomEdid(req.Edid); err != nil {
			rsp.ErrRsp(c, -3, "invalid EDID")
			return
		}
		_, err = edidHelper(c.Request.Context(), data, "apply", req.Edid)
	}
	if err != nil {
		rsp.ErrRsp(c, -4, "failed to switch EDID")
		return
	}
	rsp.OkRsp(c)
}

func customEdidListVIM4(c *gin.Context) {
	var rsp proto.Response
	entries, err := os.ReadDir(customEdidDir)
	if err != nil && !os.IsNotExist(err) {
		rsp.ErrRsp(c, -1, "get EDID failed")
		return
	}
	list := []string{}
	for _, entry := range entries {
		if _, err := readCustomEdid(entry.Name()); err == nil {
			list = append(list, entry.Name())
		}
	}
	sort.Strings(list)
	rsp.OkRspWithData(c, proto.GetCustomEdidListRsp{EdidList: list})
}

func uploadEdidVIM4(c *gin.Context) {
	var rsp proto.Response
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		rsp.ErrRsp(c, -1, "bad request")
		return
	}
	defer file.Close()
	name := header.Filename
	data, err := io.ReadAll(io.LimitReader(file, 257))
	if err != nil || !edidName.MatchString(name) || edidPresets[name] || !validEdid(data) {
		rsp.ErrRsp(c, -2, "invalid EDID file")
		return
	}
	if err = os.MkdirAll(customEdidDir, 0o755); err == nil {
		err = writeFileAtomic(filepath.Join(customEdidDir, name), data)
	}
	if err != nil {
		rsp.ErrRsp(c, -3, "save failed")
		return
	}
	rsp.OkRspWithData(c, proto.UploadEdidRsp{File: name})
}

func deleteEdidVIM4(c *gin.Context) {
	var req proto.DeleteEdidReq
	var rsp proto.Response
	if err := proto.ParseFormRequest(c, &req); err != nil || !edidName.MatchString(req.Edid) {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}
	if err := os.Remove(filepath.Join(customEdidDir, req.Edid)); err != nil {
		rsp.ErrRsp(c, -2, "delete failed")
		return
	}
	rsp.OkRsp(c)
}

func writeFileAtomic(path string, data []byte) error {
	pending, err := os.CreateTemp(filepath.Dir(path), ".edid-*")
	if err != nil {
		return err
	}
	defer os.Remove(pending.Name())
	if _, err = pending.Write(data); err == nil {
		err = pending.Chmod(0o644)
	}
	if closeErr := pending.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(pending.Name(), path)
}

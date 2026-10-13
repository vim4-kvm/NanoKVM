package vm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
)

const ScriptDirectory = "/etc/kvm/scripts"

func (s *Service) GetScripts(c *gin.Context) {
	var rsp proto.Response

	files := []string{}
	entries, err := os.ReadDir(ScriptDirectory)
	if err != nil && !os.IsNotExist(err) {
		rsp.ErrRsp(c, -1, "get scripts failed")
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && isScript(entry.Name()) {
			files = append(files, entry.Name())
		}
	}

	rsp.OkRspWithData(c, &proto.GetScriptsRsp{
		Files: files,
	})

	log.Debugf("get scripts total %d", len(files))
}

func (s *Service) UploadScript(c *gin.Context) {
	var rsp proto.Response

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		rsp.ErrRsp(c, -1, "bad request")
		return
	}
	defer file.Close()

	target, err := scriptPath(ScriptDirectory, header.Filename)
	if err != nil {
		rsp.ErrRsp(c, -2, "invalid arguments")
		return
	}

	if err = os.MkdirAll(ScriptDirectory, 0o755); err != nil {
		rsp.ErrRsp(c, -2, "create directory failed")
		return
	}

	// Write a new inode then rename so replacing a script never follows a symlink
	// or exposes a partially uploaded file to the runner.
	dst, err := os.CreateTemp(ScriptDirectory, ".upload-*")
	if err != nil {
		rsp.ErrRsp(c, -2, "create file failed")
		return
	}
	defer dst.Close()
	defer os.Remove(dst.Name())

	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(dst, file, buf)
	if err != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	if dst.Close() != nil || os.Chmod(dst.Name(), 0700) != nil || os.Rename(dst.Name(), target) != nil {
		rsp.ErrRsp(c, -2, "save failed")
		return
	}

	data := &proto.UploadScriptRsp{
		File: header.Filename,
	}
	rsp.OkRspWithData(c, data)

	log.Debugf("upload script %s success", header.Filename)
}

func (s *Service) RunScript(c *gin.Context) {
	var req proto.RunScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if req.Type != "foreground" && req.Type != "background" {
		rsp.ErrRsp(c, -1, "invalid execution type")
		return
	}
	cmd, err := scriptCommand(ScriptDirectory, req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid script")
		return
	}
	var output []byte

	if req.Type == "foreground" {
		output, err = cmd.CombinedOutput()
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
		err = cmd.Start()
		if err == nil {
			go func() {
				err := cmd.Wait()
				if err != nil {
					log.Errorf("run script %s in background failed: %s", req.Name, err)
				}
			}()
		}
	}

	if err != nil {
		log.Errorf("run script %s failed: %s", req.Name, err.Error())
		rsp.ErrRsp(c, -2, "run script failed")
		return
	}

	rsp.OkRspWithData(c, &proto.RunScriptRsp{
		Log: string(output),
	})

	log.Debugf("run script %s success", req.Name)
}

func (s *Service) DeleteScript(c *gin.Context) {
	var req proto.DeleteScriptReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	file, err := scriptPath(ScriptDirectory, req.Name)
	if err != nil {
		rsp.ErrRsp(c, -1, "invalid script")
		return
	}

	if err := os.Remove(file); err != nil {
		log.Errorf("delete script %s failed: %s", file, err)
		rsp.ErrRsp(c, -3, "delete failed")
		return
	}

	rsp.OkRsp(c)
	log.Debugf("delete script %s success", file)
}

func isScript(name string) bool {
	nameLower := strings.ToLower(name)
	if strings.HasSuffix(nameLower, ".sh") || strings.HasSuffix(nameLower, ".py") {
		return true
	}

	return false
}

func scriptPath(directory, name string) (string, error) {
	if !isScript(name) || filepath.Base(name) != name || strings.ContainsAny(name, "\\\x00") {
		return "", fmt.Errorf("invalid script name")
	}
	return filepath.Join(directory, name), nil
}

func scriptCommand(directory, name string) (*exec.Cmd, error) {
	path, err := scriptPath(directory, name)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("script must be a regular file")
	}
	interpreter := "/bin/sh"
	if strings.EqualFold(filepath.Ext(name), ".py") {
		interpreter = "python3"
	}
	if interpreter == "/bin/sh" {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		var prefix [2]byte
		_, _ = file.Read(prefix[:])
		_ = file.Close()
		if string(prefix[:]) == "#!" {
			return exec.Command(path), nil
		}
	}
	return exec.Command(interpreter, path), nil
}

//go:build vim4

package router

import (
	"NanoKVM-Server/config"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/extensions/assistant"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	assistantPython = "/var/lib/vim4-kvm/assistant/bin/python"
	assistantScript = "/opt/vim4-nanokvm/assistant/cua_webapp.py"
	assistantLog    = "/tmp/vim4-assistant.log"
	// The VIM4 gadget numbers its mice differently from the appliance, and the
	// assistant must take snapshots instead of switching the viewer to MJPEG.
	assistantEnv = []string{
		"KVM_HID_KEYBOARD=/dev/hidg0",
		"KVM_HID_MOUSE=/dev/hidg2",
		"KVM_HID_ABSOLUTE=/dev/hidg1",
		"PYTHONUNBUFFERED=1",
	}
)

// Installs python3-venv as root, then writes the original requirements file;
// the page runs pip into the application-owned venv in the web terminal.
func assistantInstallVIM4(a *assistant.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Minute)
		defer cancel()
		if err := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-system", "assistant-prepare").Run(); err != nil {
			var rsp proto.Response
			rsp.ErrRsp(c, -1, "failed to prepare the Python environment")
			return
		}
		a.Install(c)
	}
}

func assistantRunning() bool {
	return exec.Command("pgrep", "-f", assistantScript).Run() == nil
}

func assistantStartVIM4(c *gin.Context) {
	var rsp proto.Response
	if assistantRunning() {
		rsp.OkRspWithData(c, gin.H{"status": "running", "code": 0})
		return
	}
	if _, err := os.Stat(assistantPython); err != nil {
		rsp.ErrRsp(c, -1, "assistant dependencies are not installed")
		return
	}
	logFile, err := os.Create(assistantLog)
	if err != nil {
		rsp.ErrRsp(c, -1, "failed to start python server")
		return
	}
	defer logFile.Close()
	token, err := config.GetPicoclawInternalToken()
	if err != nil {
		rsp.ErrRsp(c, -1, "failed to initialize assistant capture access")
		return
	}
	cmd := exec.Command(assistantPython, assistantScript, "--auth")
	cmd.Dir = "/opt/vim4-nanokvm/assistant"
	cmd.Env = append(os.Environ(), assistantEnv...)
	cmd.Env = append(cmd.Env,
		"KVM_SNAPSHOT_TOKEN="+token,
		"KVM_SNAPSHOT_URL=http://127.0.0.1:"+strconv.Itoa(config.GetInstance().Port.Http)+"/api/stream/snapshot")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		rsp.ErrRsp(c, -1, "failed to start python server")
		return
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	deadline := time.After(30 * time.Second)
	for {
		if data, _ := os.ReadFile(assistantLog); strings.Contains(string(data), "Authentication") {
			rsp.OkRspWithData(c, gin.H{"status": "starting", "code": 0})
			return
		}
		select {
		case <-exited:
			rsp.ErrRsp(c, -2, "python server exited; see "+assistantLog)
			return
		case <-deadline:
			rsp.ErrRsp(c, -2, "python server did not start")
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

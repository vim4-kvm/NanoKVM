//go:build vim4

package router

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/extensions/tailscale"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
)

type tailscaleStatusVIM4 struct {
	Installed    bool     `json:"installed"`
	Running      bool     `json:"running"`
	BackendState string   `json:"backendState"`
	HostName     string   `json:"hostName"`
	IPs          []string `json:"ips"`
	Tailnet      string   `json:"tailnet"`
}

// Tailscale installs an apt package and runs a root daemon; only the
// argument-validated helper performs those operations.
func tailscaleHelper(ctx context.Context, action string, result interface{}) error {
	timeout := time.Minute
	if action == "install" || action == "uninstall" {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-tailscale", action).Output()
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewReader(out)).Decode(result)
}

func tailscaleStatusRsp(s tailscaleStatusVIM4) (proto.GetTailscaleStatusRsp, bool) {
	switch {
	case !s.Installed:
		return proto.GetTailscaleStatusRsp{State: proto.TailscaleNotInstall}, true
	case !s.Running:
		return proto.GetTailscaleStatusRsp{State: proto.TailscaleNotRunning}, true
	}
	state, ok := tailscale.StateMap[s.BackendState]
	rsp := proto.GetTailscaleStatusRsp{State: state, Name: s.HostName, Account: s.Tailnet}
	for _, address := range s.IPs {
		if ip := net.ParseIP(address); ip != nil && ip.To4() != nil {
			rsp.IP = ip.String()
		}
	}
	return rsp, ok
}

func tailscaleStatusVIM4Handler(c *gin.Context) {
	var rsp proto.Response
	var status tailscaleStatusVIM4
	if err := tailscaleHelper(c.Request.Context(), "status", &status); err != nil {
		rsp.ErrRsp(c, -1, "get tailscale status failed")
		return
	}
	data, ok := tailscaleStatusRsp(status)
	if !ok {
		rsp.ErrRsp(c, -1, "unknown state")
		return
	}
	rsp.OkRspWithData(c, &data)
}

func tailscaleLoginVIM4(c *gin.Context) {
	var rsp proto.Response
	var login proto.LoginTailscaleRsp
	if err := tailscaleHelper(c.Request.Context(), "login", &login); err != nil {
		rsp.ErrRsp(c, -2, "login failed")
		return
	}
	rsp.OkRspWithData(c, &login)
}

func tailscaleActionVIM4(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rsp proto.Response
		var result struct{}
		if err := tailscaleHelper(c.Request.Context(), action, &result); err != nil {
			rsp.ErrRsp(c, -1, action+" failed")
			return
		}
		rsp.OkRsp(c)
	}
}

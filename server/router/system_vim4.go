//go:build vim4

package router

import (
	"NanoKVM-Server/proto"
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"os/exec"
	"time"
)

// Keep Ubuntu privilege changes behind one argument-validated helper. The
// original UI and response contracts stay shared with the appliance target.
func systemVIM4(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rsp proto.Response
		body := []byte("{}")
		if c.Request.Method == "POST" && c.Request.ContentLength != 0 {
			var err error
			body, err = io.ReadAll(io.LimitReader(c.Request.Body, 8193))
			if err != nil || len(body) > 8192 || !json.Valid(body) {
				rsp.ErrRsp(c, -1, "invalid arguments")
				return
			}
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-system", action)
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.Output()
		if err != nil {
			rsp.ErrRsp(c, -2, "Ubuntu operation failed; check parameters and device state")
			return
		}
		var data map[string]any
		if err = json.Unmarshal(out, &data); err != nil {
			rsp.ErrRsp(c, -3, "invalid system response")
			return
		}
		if action == "wifi-get" {
			data["ssid"] = ""
			if wifi, ok := data["wifi"].(map[string]any); ok {
				if ssid, ok := wifi["ssid"].(string); ok {
					data["ssid"] = ssid
				}
			}
		}
		rsp.OkRspWithData(c, data)
	}
}

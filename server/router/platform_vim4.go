//go:build vim4

package router

import (
	"NanoKVM-Server/authn"
	"NanoKVM-Server/common"
	"NanoKVM-Server/config"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/hid"
	"NanoKVM-Server/service/storage"
	"NanoKVM-Server/service/vm"
	"context"
	"github.com/gin-gonic/gin"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Select only hardware-dependent handlers. Authentication and role middleware
// remain on the official routes, including control, MCP and multi-user APIs.
func platformHandler(route string, handler gin.HandlerFunc) gin.HandlerFunc {
	actions := map[string]string{
		"GET /network/wifi": "wifi-get", "POST /network/wifi/connect": "wifi-connect", "POST /network/wifi/disconnect": "wifi-disconnect",
		"GET /network/dns": "dns-get", "POST /network/dns": "dns-set",
		"GET /vm/ssh": "ssh-get", "POST /vm/ssh/enable": "ssh-enable", "POST /vm/ssh/disable": "ssh-disable",
		"GET /vm/mdns": "mdns-get", "POST /vm/mdns/enable": "mdns-enable", "POST /vm/mdns/disable": "mdns-disable",
		"POST /vm/hostname": "hostname-set", "POST /vm/system/reboot": "reboot",
	}
	if action, ok := actions[route]; ok {
		return systemVIM4(action)
	}
	if strings.HasPrefix(route, "POST /tailscale/") {
		action := strings.TrimPrefix(route, "POST /tailscale/")
		if action == "login" {
			return tailscaleLoginVIM4
		}
		return tailscaleActionVIM4(action)
	}
	if route == "GET /tailscale/status" {
		return tailscaleStatusVIM4Handler
	}
	switch route {
	case "POST /vm/screen":
		return setScreenVIM4
	case "GET /vm/info":
		return vm.NewService().GetUbuntuInfo
	case "GET /vm/swap":
		return func(c *gin.Context) {
			var rsp proto.Response
			rsp.OkRspWithData(c, gin.H{"size": 0, "configured": false})
		}
	case "GET /vm/hdmi":
		return func(c *gin.Context) {
			var rsp proto.Response
			rsp.OkRspWithData(c, gin.H{"enabled": true, "signal": common.GetKvmVision().HasHDMISignal(), "idleTimeout": 0, "configured": false})
		}
	case "POST /vm/hdmi/reset", "POST /vm/hdmi/enable", "POST /vm/hdmi/disable", "POST /vm/hdmi/timeout":
		return unconfiguredVIM4
	case "GET /vm/input-resolution":
		return func(c *gin.Context) {
			w, h := common.GetKvmVision().Dimensions()
			var rsp proto.Response
			rsp.OkRspWithData(c, proto.GetInputResolutionRsp{Width: int(w), Height: int(h)})
		}
	case "POST /hid/mode":
		return unconfiguredVIM4
	case "GET /hid/mode":
		return func(c *gin.Context) {
			var rsp proto.Response
			rsp.OkRspWithData(c, proto.GetHidModeRsp{Mode: "normal"})
		}
	case "GET /vm/hardware":
		return func(c *gin.Context) { var rsp proto.Response; rsp.OkRspWithData(c, gin.H{"version": "VIM4"}) }
	case "GET /vm/gpio":
		return func(c *gin.Context) {
			var rsp proto.Response
			rsp.OkRspWithData(c, gin.H{"pwr": false, "hdd": false, "configured": false})
		}
	case "GET /vm/oled":
		return func(c *gin.Context) { var rsp proto.Response; rsp.OkRspWithData(c, proto.GetOLEDRsp{}) }
	case "GET /vm/device/virtual":
		return virtualDevicesVIM4
	case "POST /vm/device/virtual":
		return toggleVirtualDeviceVIM4
	case "POST /vm/gpio", "POST /vm/oled", "POST /vm/swap", "POST /api/network/wifi", "POST /api/network/wifi/verify":
		return unconfiguredVIM4
	}
	if strings.Contains(route, "/vm/autostart") {
		return unconfiguredVIM4
	}
	return handler
}

func unconfiguredVIM4(c *gin.Context) {
	var rsp proto.Response
	rsp.ErrRsp(c, -1, "not configured on VIM4")
}

func platformRouter(r *gin.Engine) {
	streamRouterVIM4(r)
	r.GET("/api/platform", func(c *gin.Context) {
		c.JSON(200, gin.H{"platform": "vim4", "authentication": config.GetInstance().Authentication})
	})
	admin := r.Group("/api").Use(middleware.CheckToken(), middleware.RequireRole(authn.RoleAdmin))
	for path, action := range map[string]string{"/network/static-ip": "static-get", "/network/wifi/scan": "wifi-scan"} {
		admin.GET(path, systemVIM4(action))
	}
	for path, action := range map[string]string{"/network/static-ip": "static-set", "/vm/timezone": "timezone-set", "/vm/time/sync": "time-sync"} {
		admin.POST(path, systemVIM4(action))
	}
	admin.GET("/vm/edid", getEdidVIM4)
	admin.POST("/vm/edid", switchEdidVIM4)
	admin.POST("/vm/edid/upload", uploadEdidVIM4)
	admin.GET("/vm/edid/custom", customEdidListVIM4)
	admin.POST("/vm/edid/delete", deleteEdidVIM4)
	v := vm.NewService()
	admin.GET("/vm/timezone", v.GetTimeZone)
	admin.GET("/vm/time", v.GetTime)
	admin.GET("/vm/time/status", v.GetTimeStatus)
}

func gadgetLinked(function string) bool {
	_, err := os.Lstat("/sys/kernel/config/usb_gadget/kvm/configs/c.1/" + function)
	return err == nil
}
func virtualDevicesVIM4(c *gin.Context) {
	var rsp proto.Response
	rsp.OkRspWithData(c, gin.H{"network": gadgetLinked("ncm.usb0"), "media": gadgetLinked("mass_storage.disk0"), "disk": storage.DataDiskAttached(), "mic": gadgetLinked("uac2.usb0"), "isMicEnabled": gadgetLinked("uac2.usb0"), "isNetworkEnabled": gadgetLinked("ncm.usb0")})
}
func toggleVirtualDeviceVIM4(c *gin.Context) {
	var req proto.UpdateVirtualDeviceReq
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil || (req.Device != "mic" && req.Device != "disk") {
		rsp.ErrRsp(c, -1, "unsupported virtual device")
		return
	}
	state := "on"
	action := "microphone"
	enabled := gadgetLinked("uac2.usb0")
	if req.Device == "disk" {
		action = "disk"
		enabled = storage.DataDiskAttached()
	}
	if enabled {
		state = "off"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := hid.GetHid().Reenumerate(exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-media", action, state).Run); err != nil {
		rsp.ErrRsp(c, -2, "failed to switch USB device")
		return
	}
	rsp.OkRspWithData(c, proto.UpdateVirtualDeviceRsp{On: state == "on"})
}

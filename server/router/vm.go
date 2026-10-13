package router

import (
	"github.com/gin-gonic/gin"

	"NanoKVM-Server/authn"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/vm"
)

func vmRouter(r *gin.Engine) {
	service := vm.NewService()

	api := r.Group("/api").Use(middleware.CheckToken())
	admin := r.Group("/api").Use(
		middleware.CheckToken(),
		middleware.RequireRole(authn.RoleAdmin),
	)

	api.GET("/vm/info", platformHandler("GET /vm/info", service.GetInfo))             // get device information
	api.GET("/vm/hardware", platformHandler("GET /vm/hardware", service.GetHardware)) // get hardware version

	api.POST("/vm/gpio", platformHandler("POST /vm/gpio", service.SetGpio))       // update gpio
	api.GET("/vm/gpio", platformHandler("GET /vm/gpio", service.GetGpio))         // get gpio
	api.POST("/vm/screen", platformHandler("POST /vm/screen", service.SetScreen)) // update screen

	api.GET("/vm/input-region", platformHandler("GET /vm/input-region", service.GetInputRegion))
	api.POST("/vm/input-region", platformHandler("POST /vm/input-region", service.SetInputRegion))
	api.GET("/vm/input-resolution", platformHandler("GET /vm/input-resolution", service.GetInputResolution))

	admin.GET("/vm/terminal", platformHandler("GET /vm/terminal", service.Terminal)) // web terminal

	admin.GET("/vm/script", platformHandler("GET /vm/script", service.GetScripts))                   // get script
	admin.POST("/vm/script/upload", platformHandler("POST /vm/script/upload", service.UploadScript)) // upload script
	admin.POST("/vm/script/run", platformHandler("POST /vm/script/run", service.RunScript))          // run script
	admin.DELETE("/vm/script", platformHandler("DELETE /vm/script", service.DeleteScript))           // delete script

	admin.GET("/vm/device/virtual", platformHandler("GET /vm/device/virtual", service.GetVirtualDevice))      // get virtual device
	admin.POST("/vm/device/virtual", platformHandler("POST /vm/device/virtual", service.UpdateVirtualDevice)) // update virtual device

	admin.GET("/vm/memory/limit", platformHandler("GET /vm/memory/limit", service.GetMemoryLimit))   // get memory limit
	admin.POST("/vm/memory/limit", platformHandler("POST /vm/memory/limit", service.SetMemoryLimit)) // set memory limit

	admin.GET("/vm/oled", platformHandler("GET /vm/oled", service.GetOLED))   // get OLED configuration
	admin.POST("/vm/oled", platformHandler("POST /vm/oled", service.SetOLED)) // set OLED configuration

	// Only supported by PCIe version
	api.GET("/vm/hdmi", platformHandler("GET /vm/hdmi", service.GetHdmiState))                    // get HDMI state
	api.POST("/vm/hdmi/reset", platformHandler("POST /vm/hdmi/reset", service.ResetHdmi))         // reset hdmi
	admin.POST("/vm/hdmi/enable", platformHandler("POST /vm/hdmi/enable", service.EnableHdmi))    // enable hdmi
	admin.POST("/vm/hdmi/disable", platformHandler("POST /vm/hdmi/disable", service.DisableHdmi)) // disable hdmi
	admin.POST("/vm/hdmi/timeout", platformHandler("POST /vm/hdmi/timeout", service.SetHdmiIdleTimeout))

	admin.GET("/vm/ssh", platformHandler("GET /vm/ssh", service.GetSSHState))                  // get SSH state
	admin.POST("/vm/ssh/enable", platformHandler("POST /vm/ssh/enable", service.EnableSSH))    // enable SSH
	admin.POST("/vm/ssh/disable", platformHandler("POST /vm/ssh/disable", service.DisableSSH)) // disable SSH

	admin.GET("/vm/swap", platformHandler("GET /vm/swap", service.GetSwap))   // get swap file size
	admin.POST("/vm/swap", platformHandler("POST /vm/swap", service.SetSwap)) // set swap file size

	admin.GET("/vm/mouse-jiggler", platformHandler("GET /vm/mouse-jiggler", service.GetMouseJiggler))     // get mouse jiggler
	admin.POST("/vm/mouse-jiggler/", platformHandler("POST /vm/mouse-jiggler/", service.SetMouseJiggler)) // set mouse jiggler

	api.GET("/vm/hostname", platformHandler("GET /vm/hostname", service.GetHostname))     // Get Hostname
	admin.POST("/vm/hostname", platformHandler("POST /vm/hostname", service.SetHostname)) // Set Hostname

	api.GET("/vm/web-title", platformHandler("GET /vm/web-title", service.GetWebTitle))     // Get web title
	admin.POST("/vm/web-title", platformHandler("POST /vm/web-title", service.SetWebTitle)) // Set web title

	admin.GET("/vm/mdns", platformHandler("GET /vm/mdns", service.GetMdnsState))                  // get mDNS state
	admin.POST("/vm/mdns/enable", platformHandler("POST /vm/mdns/enable", service.EnableMdns))    // enable mDNS
	admin.POST("/vm/mdns/disable", platformHandler("POST /vm/mdns/disable", service.DisableMdns)) // disable mDNS

	admin.POST("/vm/tls", platformHandler("POST /vm/tls", service.SetTls)) // enable/disable TLS

	admin.GET("/vm/autostart", platformHandler("GET /vm/autostart", service.GetAutostart))                      // get autostart list
	admin.GET("/vm/autostart/:name", platformHandler("GET /vm/autostart/:name", service.GetAutostartContent))   // get autostart content
	admin.DELETE("/vm/autostart/:name", platformHandler("DELETE /vm/autostart/:name", service.DeleteAutostart)) // delete autostart script
	admin.POST("/vm/autostart/:name", platformHandler("POST /vm/autostart/:name", service.UploadAutostart))     // upload autostart script

	admin.POST("/vm/system/reboot", platformHandler("POST /vm/system/reboot", service.Reboot)) // reboot system
}

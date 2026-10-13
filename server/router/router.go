package router

import (
	"NanoKVM-Server/internal/platform"
	"fmt"
	"os"
	"path/filepath"

	"NanoKVM-Server/service/controlmode"
	"NanoKVM-Server/service/picoclaw"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func Init(r *gin.Engine) {
	web(r)
	server(r)
	log.Debugf("router init done")
}

func web(r *gin.Engine) {
	execPath, err := os.Executable()
	if err != nil {
		panic("invalid executable path")
	}

	execDir := filepath.Dir(execPath)
	webPath := fmt.Sprintf("%s/web", execDir)

	if platform.VIM4 {
		r.Use(func(c *gin.Context) {
			if c.Request.URL.Path == "/" || c.Request.URL.Path == "/index.html" {
				c.Header("Cache-Control", "no-store")
			}
			c.Next()
		})
	}
	r.Use(static.Serve("/", static.LocalFile(webPath, true)))
}

func server(r *gin.Engine) {
	control := controlmode.GetManager()
	picoclawService := picoclaw.NewService(control)

	authRouter(r)
	applicationRouter(r)
	vmRouter(r)
	streamRouter(r)
	storageRouter(r)
	networkRouter(r)
	hidRouter(r)
	controlRouter(r, control, picoclawService)
	mcpRouter(r, control, picoclawService)
	picoclawRouter(r, picoclawService)
	wsRouter(r)
	downloadRouter(r)
	extensionsRouter(r)
	platformRouter(r)
}

func LoopbackHTTPAllowedPaths() []string {
	paths := PicoclawLoopbackHTTPAllowedPaths()
	paths = append(paths, HIDLoopbackHTTPAllowedPaths()...)
	paths = append(paths, platformLoopbackHTTPPaths()...)
	return paths
}

package router

import (
	"NanoKVM-Server/authn"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/extensions/tailscale"

	"github.com/gin-gonic/gin"
)

func extensionsRouter(r *gin.Engine) {
	api := r.Group("/api/extensions").Use(
		middleware.CheckToken(),
		middleware.RequireRole(authn.RoleAdmin),
	)

	ts := tailscale.NewService()

	api.POST("/tailscale/install", platformHandler("POST /tailscale/install", ts.Install))       // install tailscale
	api.POST("/tailscale/uninstall", platformHandler("POST /tailscale/uninstall", ts.Uninstall)) // uninstall tailscale
	api.GET("/tailscale/status", platformHandler("GET /tailscale/status", ts.GetStatus))         // get tailscale status
	api.POST("/tailscale/up", platformHandler("POST /tailscale/up", ts.Up))                      // run tailscale up
	api.POST("/tailscale/down", platformHandler("POST /tailscale/down", ts.Down))                // run tailscale down
	api.POST("/tailscale/login", platformHandler("POST /tailscale/login", ts.Login))             // tailscale login
	api.POST("/tailscale/logout", platformHandler("POST /tailscale/logout", ts.Logout))          // tailscale logout
	api.POST("/tailscale/start", platformHandler("POST /tailscale/start", ts.Start))             // tailscale start
	api.POST("/tailscale/stop", platformHandler("POST /tailscale/stop", ts.Stop))                // tailscale stop
	api.POST("/tailscale/restart", platformHandler("POST /tailscale/restart", ts.Restart))       // tailscale restart
}

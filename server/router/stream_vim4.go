//go:build vim4

package router

import (
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/stream"
	"NanoKVM-Server/service/stream/direct"
	"NanoKVM-Server/service/stream/webrtc"
	"github.com/gin-gonic/gin"
)

func streamRouterVIM4(r *gin.Engine) {
	api := r.Group("/api/stream").Use(middleware.CheckToken())
	api.GET("/h264/webrtc", webrtc.Connect)
	api.GET("/h265/webrtc", webrtc.ConnectH265)
	api.GET("/h265/direct", direct.ConnectH265)
	api.GET("/audio/webrtc", webrtc.ConnectAudio)
	service := stream.NewService()
	r.GET("/api/stream/snapshot", middleware.CheckTokenOrLoopbackInternalToken(), service.Snapshot)
	api.GET("/settings", service.GetSettings)
	api.POST("/resolution", service.SetResolution)
	api.POST("/rate-control", service.SetRateControlMode)
	api.POST("/fps", service.SetFps)
	api.POST("/gop", service.SetGop)
	api.POST("/quality", service.SetQuality)
	api.POST("/bitrate", setBitrateVIM4)
	api.POST("/mode", setModeVIM4)
}

func platformLoopbackHTTPPaths() []string { return []string{"/api/stream/snapshot"} }

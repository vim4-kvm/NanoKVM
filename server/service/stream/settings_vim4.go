//go:build linux && vim4

package stream

import (
	"NanoKVM-Server/common"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/vm"
	"context"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Snapshot(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	release, _, err := vm.AcquireHdmiCaptureLeaseForRead(ctx)
	if err != nil {
		c.Status(503)
		return
	}
	defer release()
	data, err := common.GetKvmVision().Snapshot(ctx)
	if err != nil {
		c.Status(503)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(200, "image/jpeg", data)
}

func (s *Service) GetSettings(c *gin.Context) {
	var rsp proto.Response
	settings := common.GetKvmVision().VideoSettings()
	rsp.OkRspWithData(c, gin.H{"width": settings.Width, "height": settings.Height, "bitrate": settings.Bitrate, "gop": settings.GOP, "fps": settings.FPS, "quality": settings.Quality, "rateControl": settings.RateControl, "actualFPS": GetFrameRateCounter().GetFPS(), "h265Main10": common.H265Main10()})
}

func updateSettings(c *gin.Context, update func(*common.VideoSettings)) {
	var rsp proto.Response
	if err := common.GetKvmVision().UpdateVideoSettings(update); err != nil {
		log.Warnf("update video settings: %v", err)
		rsp.ErrRsp(c, -2, "failed to update video settings")
		return
	}
	rsp.OkRsp(c)
}
func invalidSettings(c *gin.Context) { var rsp proto.Response; rsp.ErrRsp(c, -1, "invalid arguments") }

func (s *Service) SetResolution(c *gin.Context) {
	var req struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	updateSettings(c, func(s *common.VideoSettings) { s.Width, s.Height = req.Width, req.Height })
}
func (s *Service) SetRateControlMode(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	updateSettings(c, func(s *common.VideoSettings) { s.RateControl = strings.ToLower(req.Mode) })
}
func (s *Service) SetFps(c *gin.Context) {
	var req struct {
		FPS int `json:"fps"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	updateSettings(c, func(s *common.VideoSettings) { s.FPS = req.FPS })
}
func (s *Service) SetGop(c *gin.Context) {
	var req struct {
		GOP int `json:"gop"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	updateSettings(c, func(s *common.VideoSettings) { s.GOP = req.GOP })
}
func (s *Service) SetQuality(c *gin.Context) {
	var req struct {
		Quality int `json:"quality"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	updateSettings(c, func(s *common.VideoSettings) {
		if req.Quality > 100 {
			s.Bitrate = req.Quality
		} else {
			s.Quality = req.Quality
		}
	})
}
func (s *Service) SetMode(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	if proto.ParseFormRequest(c, &req) != nil {
		invalidSettings(c)
		return
	}
	switch strings.ToLower(req.Mode) {
	case "mjpeg":
		SelectMJPEG()
	case "h264", "h264-webrtc", "h264-direct":
		SelectVideo(false)
	case "h265", "h265-webrtc", "h265-direct":
		SelectVideo(true)
	default:
		invalidSettings(c)
		return
	}
	var rsp proto.Response
	rsp.OkRsp(c)
}

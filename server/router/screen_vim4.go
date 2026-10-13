//go:build vim4

package router

import (
	"NanoKVM-Server/common"
	"NanoKVM-Server/proto"
	"NanoKVM-Server/service/stream"
	"github.com/gin-gonic/gin"
	"strings"
)

func selectVideoModeVIM4(mode string) bool {
	switch strings.ToLower(mode) {
	case "mjpeg":
		stream.SelectMJPEG()
	case "h264", "direct", "h264-webrtc", "h264-direct":
		stream.SelectVideo(false)
	case "h265", "h265-webrtc", "h265-direct":
		stream.SelectVideo(true)
	default:
		return false
	}
	return true
}

func setModeVIM4(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil || !selectVideoModeVIM4(req.Mode) {
		rsp.ErrRsp(c, -1, "unsupported video mode")
		return
	}
	rsp.OkRsp(c)
}

func setBitrateVIM4(c *gin.Context) {
	var req struct {
		Bitrate int `json:"bitrate"`
	}
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil {
		rsp.ErrRsp(c, -1, "invalid bitrate")
		return
	}
	if err := common.GetKvmVision().UpdateVideoSettings(func(s *common.VideoSettings) { s.Bitrate = req.Bitrate }); err != nil {
		rsp.ErrRsp(c, -2, "failed to update bitrate")
		return
	}
	rsp.OkRsp(c)
}

// Keep the original screen API alongside the explicit capture controls. The
// legacy resolution menu sizes the browser viewport; capture dimensions are
// changed only by /stream/resolution, which invokes the board helper.
func setScreenVIM4(c *gin.Context) {
	var req proto.SetScreenReq
	var rsp proto.Response
	if proto.ParseFormRequest(c, &req) != nil {
		rsp.ErrRsp(c, -1, "invalid screen settings")
		return
	}
	switch req.Type {
	case "type":
		modes := []string{"mjpeg", "h264", "h265"}
		if req.Value < 0 || req.Value >= len(modes) {
			rsp.ErrRsp(c, -1, "unsupported screen type")
			return
		}
		selectVideoModeVIM4(modes[req.Value])
	case "resolution":
		switch req.Value {
		case 0, 480, 600, 720, 1080, 2160:
		default:
			rsp.ErrRsp(c, -1, "unsupported display resolution")
			return
		}
	case "quality", "gop", "fps":
		err := common.GetKvmVision().UpdateVideoSettings(func(s *common.VideoSettings) {
			switch req.Type {
			case "quality":
				if req.Value > 100 {
					s.Bitrate = req.Value
				} else {
					s.Quality = req.Value
				}
			case "gop":
				s.GOP = req.Value
			case "fps":
				s.FPS = req.Value
			}
		})
		if err != nil {
			rsp.ErrRsp(c, -2, "failed to update screen settings")
			return
		}
	default:
		rsp.ErrRsp(c, -1, "unsupported screen setting")
		return
	}
	rsp.OkRsp(c)
}

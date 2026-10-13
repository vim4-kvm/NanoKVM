//go:build linux && vim4

package stream

import (
	"NanoKVM-Server/common"
	"time"
)

func capturePaced() bool { return true }
func RequestKeyframe()   { common.GetKvmVision().RequestKeyframe() }
func SelectVideo(h265 bool) {
	mode := uint8(common.STREAM_TYPE_H264_WEBRTC)
	if h265 {
		mode = common.STREAM_TYPE_H265_WEBRTC
	}
	common.GetKvmVision().SetStreamType(mode)
}
func stopVideo(h265 bool) {
	mode := uint8(common.STREAM_TYPE_H264_WEBRTC)
	if h265 {
		mode = common.STREAM_TYPE_H265_WEBRTC
	}
	common.GetKvmVision().StopVideoFor(mode)
}
func readVideo(v *common.KvmVision, s *common.Screen, h265 bool) ([]byte, int, time.Duration, bool) {
	if h265 {
		return v.ReadH265Sample(s.Width, s.Height, s.BitRate)
	}
	return v.ReadH264Sample(s.Width, s.Height, s.BitRate)
}
func SelectMJPEG() { common.GetKvmVision().SetStreamType(common.STREAM_TYPE_MJPEG) }
func StopMJPEG()   { common.GetKvmVision().StopVideoFor(common.STREAM_TYPE_MJPEG) }

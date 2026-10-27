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
func readVideo(v *common.KvmVision, s *common.Screen, h265 bool) ([]byte, int, time.Duration, bool, [4]byte) {
	frame, result := v.ReadStreamFrame(h265)
	return frame.Data, result, frame.PTS, frame.HasPTS, frame.ColorSpace
}
func SelectMJPEG() { common.GetKvmVision().SetStreamType(common.STREAM_TYPE_MJPEG) }
func StopMJPEG()   { common.GetKvmVision().StopVideoFor(common.STREAM_TYPE_MJPEG) }

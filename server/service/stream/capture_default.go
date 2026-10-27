//go:build !vim4

package stream

import (
	"NanoKVM-Server/common"
	"time"
)

func capturePaced() bool { return false }
func RequestKeyframe()   {}
func SelectVideo(bool)   {}
func stopVideo(bool)     {}
func SelectMJPEG()       {}
func StopMJPEG()         {}
func readVideo(v *common.KvmVision, s *common.Screen, h265 bool) ([]byte, int, time.Duration, bool, [4]byte) {
	if h265 {
		return nil, -2, 0, false, [4]byte{}
	}
	d, r := v.ReadH264(s.Width, s.Height, s.BitRate)
	return d, r, 0, false, [4]byte{}
}

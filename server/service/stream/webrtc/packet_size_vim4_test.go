//go:build vim4

package webrtc

import (
	"testing"
)

func TestBothCodecsRespectConfiguredPacketBudget(t *testing.T) {
	for _, test := range []struct {
		value  string
		budget int
	}{{"", 1200}, {"1400", 1400}, {"1000", 1000}, {"999", 1200}, {"1401", 1200}, {"invalid", 1200}} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("VIM4_RTP_PACKET_SIZE", test.value)
			for _, manager := range []*WebRTCManager{NewWebRTCManager(), newH265Manager()} {
				data := make([]byte, 4096)
				data[0] = 0x65
				if manager.h265 {
					data[0], data[1] = 0x26, 0x01
				}
				packets := manager.videoPacketizer.Packetize(data, 1500)
				if len(packets) < 2 {
					t.Fatal("oversized frame not fragmented")
				}
				if packets[0].MarshalSize() != test.budget {
					t.Fatalf("h265=%t size=%d want=%d", manager.h265, packets[0].MarshalSize(), test.budget)
				}
				for _, packet := range packets {
					if packet.MarshalSize() > test.budget {
						t.Fatal("packet exceeds configured budget")
					}
				}
			}
		})
	}
}

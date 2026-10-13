//go:build vim4

package webrtc

import (
	"os"
	"strconv"
)

// This is the RTP packetizer budget before extensions and SRTP, not link MTU.
// Keep the conservative default for VPN/TURN paths. A measured LAN deployment
// can opt in without changing the UI or enlarging packets for every install.
func videoPacketSize() uint16 {
	size, err := strconv.Atoi(os.Getenv("VIM4_RTP_PACKET_SIZE"))
	if err != nil || size < 1000 || size > 1400 {
		return 1200
	}
	return uint16(size)
}

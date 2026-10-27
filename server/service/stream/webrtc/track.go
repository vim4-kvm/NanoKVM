package webrtc

import (
	"github.com/pion/rtp"
)

const colorSpaceURI = "http://www.webrtc.org/experiments/rtp-hdrext/color-space"

func videoPacketsWithExtensions(packets []*rtp.Packet, delayID, colorID uint32, color [4]byte) ([]*rtp.Packet, error) {
	if id := delayID; id != 0 && len(packets) > 0 {
		// Playout delay is expressed in 10 ms units; the receiver applies it
		// per frame, so only the first packet needs the extension. Only send it
		// when the browser negotiated it for this sender.
		first := packets[0].Clone()
		if err := first.SetExtension(uint8(id), []byte{0, 0, 0}); err != nil {
			return nil, err
		}
		packets = append([]*rtp.Packet{first}, packets[1:]...)
	}
	if id := colorID; id != 0 && color != [4]byte{} && len(packets) > 0 {
		// The final packet identifies this complete frame. Headers are shared
		// by viewers with different negotiated extension IDs, so clone first.
		packets = append([]*rtp.Packet(nil), packets...)
		last := len(packets) - 1
		packet := packets[last].Clone()
		if err := packet.SetExtension(uint8(id), color[:]); err != nil {
			return nil, err
		}
		packets[last] = packet
	}
	return packets, nil
}

func (t *Track) writeVideoPackets(packets []*rtp.Packet, color [4]byte) error {
	packets, err := videoPacketsWithExtensions(packets, t.delayID.Load(), t.colorID.Load(), color)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		if err := t.video.WriteRTP(packet); err != nil {
			return err
		}
	}

	return nil
}

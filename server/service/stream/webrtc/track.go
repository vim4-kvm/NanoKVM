package webrtc

import (
	"github.com/pion/rtp"
)

func (t *Track) writeVideoPackets(packets []*rtp.Packet) error {
	for _, packet := range packets {
		if id := t.delayID.Load(); id != 0 {
			packet = packet.Clone()
			// Playout delay is expressed in 10 ms units. Only send this
			// extension when the browser negotiated it for this sender.
			if err := packet.SetExtension(uint8(id), []byte{0, 0, 0}); err != nil {
				return err
			}
		}
		if err := t.video.WriteRTP(packet); err != nil {
			return err
		}
	}

	return nil
}

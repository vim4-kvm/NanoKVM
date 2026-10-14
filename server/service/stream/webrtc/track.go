package webrtc

import (
	"github.com/pion/rtp"
)

func (t *Track) writeVideoPackets(packets []*rtp.Packet) error {
	if id := t.delayID.Load(); id != 0 && len(packets) > 0 {
		// Playout delay is expressed in 10 ms units; the receiver applies it
		// per frame, so only the first packet needs the extension. Only send it
		// when the browser negotiated it for this sender.
		first := packets[0].Clone()
		if err := first.SetExtension(uint8(id), []byte{0, 0, 0}); err != nil {
			return err
		}
		packets = append([]*rtp.Packet{first}, packets[1:]...)
	}
	for _, packet := range packets {
		if err := t.video.WriteRTP(packet); err != nil {
			return err
		}
	}

	return nil
}

package webrtc

import (
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

func (c *Client) manager() *WebRTCManager {
	if c.h265 {
		return h265Manager
	}
	return getManager()
}

func newH265Manager() *WebRTCManager {
	m := NewWebRTCManager()
	m.h265 = true
	m.videoPacketizer = rtp.NewPacketizer(videoPacketSize(), 116, 0x1234ABCD, &codecs.H265Payloader{}, rtp.NewRandomSequencer(), 90000)
	return m
}

func registerH265(engine *webrtc.MediaEngine) error {
	feedback := []webrtc.RTCPFeedback{{Type: "goog-remb"}, {Type: "ccm", Parameter: "fir"}, {Type: "nack"}, {Type: "nack", Parameter: "pli"}}
	for _, codec := range []webrtc.RTPCodecParameters{
		{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH265, ClockRate: 90000, RTCPFeedback: feedback}, PayloadType: 116},
		{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeRTX, ClockRate: 90000, SDPFmtpLine: "apt=116"}, PayloadType: 117},
	} {
		if err := engine.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	}
	return nil
}

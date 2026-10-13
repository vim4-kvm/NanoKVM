package webrtc

import (
	"NanoKVM-Server/service/stream"
	"encoding/json"
	"github.com/pion/rtcp"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	log "github.com/sirupsen/logrus"

	"sync"
)

const h264SDPFmtpLine = "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=4d001f"

func NewClient(ws *websocket.Conn, videoConn *webrtc.PeerConnection) *Client {
	return &Client{
		ws:    ws,
		video: videoConn,
		mutex: sync.Mutex{},
	}
}

func (c *Client) Close() {
	if c.video != nil {
		if err := c.video.Close(); err != nil {
			log.Debugf("failed to close video peer connection: %s", err)
		}
	}

	if c.ws != nil {
		if err := c.ws.Close(); err != nil {
			log.Debugf("failed to close websocket: %s", err)
		}
	}
}

func (c *Client) WriteMessage(event string, data string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	message := &Message{
		Event: event,
		Data:  data,
	}

	if err := c.ws.WriteJSON(message); err != nil {
		log.Errorf("failed to send message %s: %v", event, err)
		return err
	}

	log.Debugf("sent message %s", event)
	return nil
}

func (c *Client) ReadMessage() (*Message, error) {
	_, raw, err := c.ws.ReadMessage()
	if err != nil {
		log.Errorf("failed to read message: %v", err)
		return nil, err
	}

	var message Message
	if err := json.Unmarshal(raw, &message); err != nil {
		log.Errorf("failed to unmarshal message: %v", err)
		return nil, nil
	}

	return &message, nil
}

func (c *Client) AddTrack() error {
	capability := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: h264SDPFmtpLine}
	if c.h265 {
		capability = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH265, ClockRate: 90000}
	}
	// video track
	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		capability,
		"video",
		"pion-video",
	)
	if err != nil {
		log.Errorf("failed to create video track: %s", err)
		return err
	}

	videoSender, err := c.video.AddTrack(videoTrack)
	if err != nil {
		log.Errorf("failed to add video track: %s", err)
		return err
	}
	go startRTCPReader(videoSender)

	track := &Track{
		video: videoTrack,
	}

	c.mutex.Lock()
	c.track = track
	c.mutex.Unlock()

	return nil
}

func startRTCPReader(sender *webrtc.RTPSender) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			log.Debugf("RTCP reader error: %v", err)
			return
		}
		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				stream.RequestKeyframe()
			}
		}
	}
}

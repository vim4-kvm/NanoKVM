package webrtc

import (
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type WebRTCManager struct {
	h265            bool
	clients         map[*websocket.Conn]*Client
	clientSnapshot  atomic.Pointer[[]*Client]
	videoPacketizer rtp.Packetizer
	videoSending    bool
	mutex           sync.Mutex
	viewerVersion   uint64
}

type Client struct {
	h265    bool
	ws      *websocket.Conn
	video   *webrtc.PeerConnection
	track   *Track
	batcher *udpBatcher
	mutex   sync.Mutex
}

func (c *Client) WsConn() *websocket.Conn {
	return c.ws
}

type SignalingHandler struct {
	client         *Client
	mutex          sync.Mutex
	unregisterMode func()
	closed         bool
}

type Track struct {
	video   *webrtc.TrackLocalStaticRTP
	delayID atomic.Uint32
	colorID atomic.Uint32
}

type Message struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

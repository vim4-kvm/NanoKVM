package webrtc

import (
	"NanoKVM-Server/config"
	"NanoKVM-Server/internal/platform"
	"NanoKVM-Server/middleware"
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pion/dtls/v3"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
	log "github.com/sirupsen/logrus"
)

var (
	upgrader = websocket.Upgrader{
		WriteBufferSize: 256 * 1024,
		CheckOrigin:     middleware.CheckWebSocketOrigin,
	}
	globalManager *WebRTCManager
	managerOnce   sync.Once
	h265Manager   = newH265Manager()
)

func getManager() *WebRTCManager {
	managerOnce.Do(func() {
		globalManager = NewWebRTCManager()
	})
	return globalManager
}

func Connect(c *gin.Context) {
	connectVideo(c, false)
}

func ConnectH265(c *gin.Context) { connectVideo(c, true) }

func connectVideo(c *gin.Context, h265 bool) {
	// create WebSocket connection
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("failed to create h264 websocket: %s", err)
		return
	}
	stopSessionWatcher := middleware.WatchWebSocket(c.Request.Context(), wsConn)
	defer stopSessionWatcher()
	defer func() {
		_ = wsConn.Close()
		log.Debugf("h264 websocket disconnected: %s", c.ClientIP())
	}()
	log.Debugf("h264 websocket connected: %s", c.ClientIP())

	var zeroTime time.Time
	_ = wsConn.SetReadDeadline(zeroTime)

	// create video connection
	iceServers := createICEServers()

	mediaEngine, err := createMediaEngine()
	if err != nil {
		log.Errorf("failed to create h264 media engine: %s", err)
		return
	}
	if h265 {
		if err := registerH265(mediaEngine); err != nil {
			return
		}
	}

	batcher, err := newUDPBatcher()
	if err != nil {
		log.Errorf("failed to create video sender: %s", err)
		return
	}
	videoConn, err := createPeerConnection(iceServers, mediaEngine, batcher)
	if err != nil {
		log.Errorf("failed to create h264 video peer connection: %s", err)
		return
	}
	defer func() {
		_ = videoConn.Close()
		log.Debugf("h264 video peer disconnected: %s", c.ClientIP())
	}()

	// create client
	client := NewClient(wsConn, videoConn)
	client.h265 = h265
	client.batcher = batcher
	if err := client.AddTrack(); err != nil {
		log.Errorf("failed to add track: %s", err)
		return
	}

	// handle signaling
	signalingHandler := NewSignalingHandler(client)
	defer signalingHandler.Close()
	signalingHandler.RegisterCallbacks()
	if err := sendICEServers(client, iceServers); err != nil {
		log.Errorf("failed to send ICE servers: %s", err)
		return
	}

	// read and wait
	for {
		message, err := client.ReadMessage()
		if err != nil {
			return
		}
		if message != nil {
			if err := signalingHandler.HandleMessage(message); err != nil {
				log.Errorf("failed to handle signaling message: %s", err)
			}
		}
	}
}

func createICEServers() []webrtc.ICEServer {
	var iceServers []webrtc.ICEServer

	conf := config.GetInstance()

	if conf.Stun != "" && conf.Stun != "disable" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: []string{"stun:" + conf.Stun},
		})
	}

	if conf.Turn.TurnAddr != "" && conf.Turn.TurnUser != "" && conf.Turn.TurnCred != "" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{"turn:" + conf.Turn.TurnAddr},
			Username:   conf.Turn.TurnUser,
			Credential: conf.Turn.TurnCred,
		})
	}

	return iceServers
}

type clientICEServer struct {
	URLs       []string    `json:"urls"`
	Username   string      `json:"username,omitempty"`
	Credential interface{} `json:"credential,omitempty"`
}

func sendICEServers(client *Client, iceServers []webrtc.ICEServer) error {
	clientServers := make([]clientICEServer, 0, len(iceServers))
	for _, server := range iceServers {
		clientServers = append(clientServers, clientICEServer{
			URLs:       server.URLs,
			Username:   server.Username,
			Credential: server.Credential,
		})
	}

	data, err := json.Marshal(clientServers)
	if err != nil {
		return err
	}

	return client.WriteMessage("ice-servers", string(data))
}

func createMediaEngine() (*webrtc.MediaEngine, error) {
	mediaEngine := &webrtc.MediaEngine{}

	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		log.Errorf("failed to register default codecs: %s", err)
		return nil, err
	}
	if platform.VIM4 {
		if err := mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: colorSpaceURI}, webrtc.RTPCodecTypeVideo); err != nil {
			return nil, err
		}
		if err := mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"}, webrtc.RTPCodecTypeVideo); err != nil {
			return nil, err
		}
	}

	return mediaEngine, nil
}

func createPeerConnection(iceServers []webrtc.ICEServer, mediaEngine *webrtc.MediaEngine, batcher *udpBatcher) (*webrtc.PeerConnection, error) {
	settingEngine := webrtc.SettingEngine{}
	if batcher != nil {
		settingEngine.SetNet(batcher)
	}
	settingEngine.SetSRTPProtectionProfiles(
		dtls.SRTP_AEAD_AES_128_GCM,
		dtls.SRTP_AES128_CM_HMAC_SHA1_80,
	)
	// The DTLS server selects the SRTP profile. Browsers prefer AES-CM/HMAC,
	// which costs twice as much per packet here as hardware-accelerated GCM.
	if err := settingEngine.SetAnsweringDTLSRole(webrtc.DTLSRoleServer); err != nil {
		return nil, err
	}

	apiOptions := []func(api *webrtc.API){
		webrtc.WithSettingEngine(settingEngine),
	}
	if mediaEngine != nil {
		registry := &interceptor.Registry{}
		if err := webrtc.ConfigureNack(mediaEngine, registry); err != nil {
			return nil, err
		}
		if err := webrtc.ConfigureRTCPReports(registry); err != nil {
			return nil, err
		}
		apiOptions = append(apiOptions, webrtc.WithInterceptorRegistry(registry))
		apiOptions = append(apiOptions, webrtc.WithMediaEngine(mediaEngine))
	}

	api := webrtc.NewAPI(apiOptions...)

	return api.NewPeerConnection(webrtc.Configuration{
		ICEServers:   iceServers,
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	})
}

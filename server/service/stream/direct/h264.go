package direct

import (
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/stream"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const pongWait = 30 * time.Second

var (
	streamer     = newStreamer()
	h265Streamer = &Streamer{clients: make(map[*client]struct{}), h265: true}
	upgrader     = websocket.Upgrader{
		WriteBufferSize: 256 * 1024,
		CheckOrigin:     middleware.CheckWebSocketOrigin,
	}
)

func Connect(c *gin.Context) {
	connect(c, streamer)
}

func ConnectH265(c *gin.Context) { connect(c, h265Streamer) }

func connect(c *gin.Context, streamer *Streamer) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("failed to upgrade to websocket: %s", err)
		return
	}
	stopSessionWatcher := middleware.WatchWebSocket(c.Request.Context(), ws)
	defer stopSessionWatcher()
	client := newClient(ws)
	if flowWindow, err := strconv.Atoi(c.Query("flow")); err == nil && flowWindow > 0 {
		client.queue.enableFlowControl(flowWindow)
	}
	defer func() {
		streamer.removeClient(client)
		client.close()
		client.wait()
		log.Debugf("h264 websocket disconnected: %s", ws.RemoteAddr())
	}()
	log.Debugf("h264 websocket connected: %s", ws.RemoteAddr())

	ws.SetReadLimit(64)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	streamer.addClient(client)

	if !streamer.h265 {
		unregisterMode := stream.RegisterH264Mode(stream.H264ModeDirect)
		defer unregisterMode()
	}

	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			log.Debugf("failed to read message (client disconnected): %s", err)
			return
		}
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		client.handleControl(messageType, data)
	}
}

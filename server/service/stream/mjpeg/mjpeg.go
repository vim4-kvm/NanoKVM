package mjpeg

import (
	"time"

	"github.com/gin-gonic/gin"
)

var streamer = NewStreamer()

type LatestFrame struct {
	Data       []byte
	Width      uint16
	Height     uint16
	CapturedAt time.Time
}

func Connect(c *gin.Context) {
	c.Header("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Pragma", "no-cache")
	c.Header("X-Server-Date", time.Now().Format(time.RFC1123))

	frames := streamer.AddClient(c)
	defer streamer.RemoveClient(c)

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case frame := <-frames:
			if err := writeFrame(c, frame); err != nil {
				return
			}
		}
	}
}

func GetLatestFrame() (LatestFrame, bool) {
	return streamer.getLatestFrame()
}

func EnableLatestFrameCache() {
	streamer.enableLatestFrameCache()
}

func DisableLatestFrameCache() {
	streamer.disableLatestFrameCache()
}

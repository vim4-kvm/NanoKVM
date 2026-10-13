package mjpeg

import (
	"github.com/gin-gonic/gin"
	"testing"
)

func TestSlowViewerReceivesLatestJPEGWithoutBlockingOthers(t *testing.T) {
	s := NewStreamer()
	slow, fast := make(chan []byte, 1), make(chan []byte, 1)
	s.clients[&gin.Context{}] = slow
	s.clients[&gin.Context{}] = fast
	s.broadcast([]byte("first"))
	if got := string(<-fast); got != "first" {
		t.Fatal(got)
	}
	// The slow viewer has not read its first frame. It must not stall the next.
	s.broadcast([]byte("second"))
	if got := string(<-fast); got != "second" {
		t.Fatal(got)
	}
	if got := string(<-slow); got != "second" {
		t.Fatal(got)
	}
}

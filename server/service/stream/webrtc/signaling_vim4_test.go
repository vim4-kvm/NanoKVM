//go:build linux && vim4

package webrtc

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Exercise the actual handler envelopes without exchanging ICE candidates:
// SDP negotiation must not require a board or start hardware capture.
func TestIndependentVideoAndAudioSignaling(t *testing.T) {
	for _, test := range []struct {
		name, event, codec string
		handler            gin.HandlerFunc
		kind               webrtc.RTPCodecType
	}{
		{"h264", "video", "H264", Connect, webrtc.RTPCodecTypeVideo},
		{"h265", "video", "H265", ConnectH265, webrtc.RTPCodecTypeVideo},
		{"audio", "audio", "opus", ConnectAudio, webrtc.RTPCodecTypeAudio},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := &webrtc.MediaEngine{}
			if test.name == "h265" {
				if err := registerH265(engine); err != nil {
					t.Fatal(err)
				}
			} else if err := engine.RegisterDefaultCodecs(); err != nil {
				t.Fatal(err)
			}
			browser, err := webrtc.NewAPI(webrtc.WithMediaEngine(engine)).NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				t.Fatal(err)
			}
			defer browser.Close()
			direction := webrtc.RTPTransceiverDirectionRecvonly
			if test.kind == webrtc.RTPCodecTypeAudio {
				direction = webrtc.RTPTransceiverDirectionSendrecv
			}
			if _, err = browser.AddTransceiverFromKind(test.kind, webrtc.RTPTransceiverInit{Direction: direction}); err != nil {
				t.Fatal(err)
			}
			offer, err := browser.CreateOffer(nil)
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.GET("/stream", test.handler)
			server := httptest.NewServer(router)
			defer server.Close()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/stream", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			var initial Message
			if err = ws.ReadJSON(&initial); err != nil || initial.Event != "ice-servers" {
				t.Fatalf("ICE config: %+v %v", initial, err)
			}
			data, _ := json.Marshal(offer)
			if err = ws.WriteJSON(Message{Event: test.event + "-offer", Data: string(data)}); err != nil {
				t.Fatal(err)
			}
			for {
				var response Message
				if err = ws.ReadJSON(&response); err != nil {
					t.Fatal(err)
				}
				if response.Event != test.event+"-answer" {
					continue
				}
				var answer webrtc.SessionDescription
				if err = json.Unmarshal([]byte(response.Data), &answer); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(answer.SDP, test.codec+"/") {
					t.Fatalf("codec missing from answer: %s", answer.SDP)
				}
				forbidden := "m=audio"
				if test.kind == webrtc.RTPCodecTypeAudio {
					forbidden = "m=video"
					if !strings.Contains(answer.SDP, "a=sendrecv") {
						t.Fatalf("microphone unavailable: %s", answer.SDP)
					}
				}
				if strings.Contains(answer.SDP, forbidden) {
					t.Fatalf("sessions are not independent: %s", answer.SDP)
				}
				break
			}
		})
	}
}

//go:build linux && vim4

package webrtc

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"NanoKVM-Server/common"
	"NanoKVM-Server/middleware"
	"NanoKVM-Server/service/stream/opus"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
	log "github.com/sirupsen/logrus"
)

type audioViewer struct {
	peer    *webrtc.PeerConnection
	track   *webrtc.TrackLocalStaticSample
	packets chan audioPacket
	done    chan struct{}
}

type audioPacket struct {
	data     []byte
	sequence uint64
}

func (p audioPacket) sample(previous uint64) media.Sample {
	var dropped uint64
	if previous != 0 && p.sequence > previous {
		dropped = p.sequence - previous - 1
	}
	// Pion advances both sequence numbers and the RTP clock for missing packets.
	// A new viewer starts at live audio without inventing preceding packet loss.
	if dropped > 65535 {
		dropped = 65535
	}
	return media.Sample{Data: p.data, Duration: 20 * time.Millisecond, PrevDroppedPackets: uint16(dropped)}
}

// All video modes share this sole HDMI reader. Slow audio viewers have bounded
// queues and cannot stall ALSA capture or steal packets from another viewer.
var audioHub = struct {
	sync.Mutex
	viewers  map[*audioViewer]bool
	running  bool
	sequence uint64
}{viewers: make(map[*audioViewer]bool)}

var microphoneOwner = make(chan struct{}, 1)

// ConnectAudio uses the native JSON envelope with audio-offer/answer/candidate.
// The audio peer supports Opus sendrecv; no video peer or encoder is opened.
func ConnectAudio(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	stopWatcher := middleware.WatchWebSocket(c.Request.Context(), ws)
	defer stopWatcher()
	servers := createICEServers()
	engine, err := createMediaEngine()
	if err != nil {
		return
	}
	peer, err := createPeerConnection(servers, engine, nil)
	if err != nil {
		return
	}
	defer peer.Close()
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "pion-audio")
	if err != nil {
		return
	}
	sender, err := peer.AddTrack(track)
	if err != nil {
		return
	}
	go func() {
		for {
			if _, _, err := sender.ReadRTCP(); err != nil {
				return
			}
		}
	}()
	client := NewClient(ws, peer)
	viewer := &audioViewer{peer: peer, track: track, packets: make(chan audioPacket, 4), done: make(chan struct{})}
	defer func() {
		audioHub.Lock()
		close(viewer.done)
		delete(audioHub.viewers, viewer)
		audioHub.Unlock()
	}()
	go viewer.write()
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		data, err := json.Marshal(candidate.ToJSON())
		if err == nil {
			_ = client.WriteMessage("audio-candidate", string(data))
		}
	})
	peer.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		audioHub.Lock()
		defer audioHub.Unlock()
		select {
		case <-viewer.done:
			return
		default:
		}
		if state != webrtc.ICEConnectionStateConnected && state != webrtc.ICEConnectionStateCompleted {
			delete(audioHub.viewers, viewer)
			return
		}
		audioHub.viewers[viewer] = true
		if !audioHub.running {
			audioHub.running = true
			go sendHDMIAudio()
		}
	})
	peer.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if remote.Kind() == webrtc.RTPCodecTypeAudio && strings.EqualFold(remote.Codec().MimeType, webrtc.MimeTypeOpus) {
			receiveMicrophone(remote)
		}
	})
	if err := sendICEServers(client, servers); err != nil {
		return
	}
	for {
		message, err := client.ReadMessage()
		if err != nil {
			return
		}
		if message == nil {
			continue
		}
		switch message.Event {
		case "audio-offer":
			var offer webrtc.SessionDescription
			if err = json.Unmarshal([]byte(message.Data), &offer); err != nil {
				return
			}
			if err = peer.SetRemoteDescription(offer); err != nil {
				return
			}
			answer, answerErr := peer.CreateAnswer(nil)
			if answerErr != nil {
				return
			}
			if err = peer.SetLocalDescription(answer); err != nil {
				return
			}
			data, marshalErr := json.Marshal(answer)
			if marshalErr != nil {
				return
			}
			err = client.WriteMessage("audio-answer", string(data))
		case "audio-candidate":
			var candidate webrtc.ICECandidateInit
			if err = json.Unmarshal([]byte(message.Data), &candidate); err == nil {
				err = peer.AddICECandidate(candidate)
			}
		case "heartbeat":
			err = client.WriteMessage("heartbeat", "")
		}
		if err != nil {
			return
		}
	}
}

func sendHDMIAudio() {
	vision := common.GetKvmVision()
	for {
		audioHub.Lock()
		if len(audioHub.viewers) == 0 {
			vision.StopAudio()
			audioHub.running = false
			audioHub.Unlock()
			return
		}
		audioHub.Unlock()
		data, result := vision.ReadAudio()
		if result < 0 || len(data) == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		audioHub.Lock()
		audioHub.sequence++
		packet := audioPacket{data: data, sequence: audioHub.sequence}
		for viewer := range audioHub.viewers {
			select {
			case viewer.packets <- packet:
			default:
			}
		}
		audioHub.Unlock()
	}
}

func (v *audioViewer) write() {
	var previous uint64
	for {
		select {
		case <-v.done:
			return
		case data := <-v.packets:
			if err := v.track.WriteSample(data.sample(previous)); err != nil {
				_ = v.peer.Close()
				return
			}
			previous = data.sequence
		}
	}
}

func receiveMicrophone(track *webrtc.TrackRemote) {
	// The USB gadget exposes one PCM stream. A second sender must not share
	// the decoder state or stop the first sender's aplay process on disconnect.
	select {
	case microphoneOwner <- struct{}{}:
	default:
		return
	}
	defer func() { <-microphoneOwner }()
	player := opus.GetAudioInputPlayer()
	if err := player.Start(); err != nil {
		log.Errorf("start USB microphone: %v", err)
		return
	}
	defer player.Stop()
	builder := samplebuilder.New(20, &opus.Packet{}, 48000)
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		builder.Push(packet)
		for sample := builder.Pop(); sample != nil; sample = builder.Pop() {
			if err := player.DecodeAndWrite(sample.Data); err != nil {
				return
			}
		}
	}
}

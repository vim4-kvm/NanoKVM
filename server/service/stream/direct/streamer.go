package direct

import (
	"NanoKVM-Server/service/stream"
	"NanoKVM-Server/service/vm"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

type Streamer struct {
	h265           bool
	mutex          sync.Mutex
	clients        map[*client]struct{}
	clientSnapshot atomic.Pointer[[]*client]
	running        bool
	viewerVersion  uint64
}

func newStreamer() *Streamer {
	s := &Streamer{
		clients: make(map[*client]struct{}),
	}
	s.updateClientSnapshotLocked()

	return s
}

func (s *Streamer) addClient(client *client) {
	stream.SelectVideo(s.h265)
	stream.RequestKeyframe()
	client.start()

	s.mutex.Lock()
	s.clients[client] = struct{}{}
	count := s.updateClientSnapshotLocked()
	s.viewerVersion++
	version := s.viewerVersion
	start := !s.running
	if start {
		s.running = true
	}
	s.mutex.Unlock()
	vm.UpdateHdmiViewerSnapshot(s.viewerSource(), count, version)

	if start {
		go s.run()
		log.Debug("h264 stream started")
	}
}

func (s *Streamer) removeClient(client *client) {
	s.mutex.Lock()
	if _, exists := s.clients[client]; !exists {
		s.mutex.Unlock()
		return
	}

	delete(s.clients, client)
	count := s.updateClientSnapshotLocked()
	s.viewerVersion++
	version := s.viewerVersion
	s.mutex.Unlock()
	client.close()
	vm.UpdateHdmiViewerSnapshot(s.viewerSource(), count, version)

	log.Debugf("h264 websocket disconnected, remaining clients: %d", count)
}

func (s *Streamer) viewerSource() string {
	if s.h265 {
		return "h265-direct"
	}
	return "direct"
}

func (s *Streamer) updateClientSnapshotLocked() int {
	clients := make([]*client, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.clientSnapshot.Store(&clients)

	return len(clients)
}

func (s *Streamer) getClients() []*client {
	clients := s.clientSnapshot.Load()
	if clients == nil {
		return nil
	}

	return *clients
}

func (s *Streamer) run() {
	var subscription *stream.H264Subscription
	if s.h265 {
		subscription = stream.SubscribeH265()
	} else {
		subscription = stream.SubscribeH264()
	}
	defer subscription.Close()

	for {
		frame, ok := subscription.Next()
		if !ok {
			return
		}
		clients := s.getClients()
		if len(clients) == 0 {
			if s.stopIfIdle() {
				log.Debug("h264 stream stopped due to no clients")
				return
			}
			continue
		}

		if !hasCaptureDemand(clients) {
			// Capture continues for shared WebRTC viewers. Frames skipped here
			// break Direct prediction chains even if its queue later has room.
			for _, client := range clients {
				client.queue.discardPredictionChain()
			}
			stream.RequestKeyframe()
			continue
		}

		mode := stream.CaptureModeDirect
		if s.h265 {
			mode = "h265-direct"
		}
		stream.UpdateCaptureStatus(mode, frame.Result)
		if frame.Result < 0 || len(frame.Data) == 0 {
			continue
		}

		outbound := newOutboundFrame(frame.IsKeyframe(), frame.Timestamp, frame.Data)
		for _, client := range clients {
			client.offer(outbound)
		}
	}
}

func (s *Streamer) stopIfIdle() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.clients) > 0 {
		return false
	}

	s.running = false
	return true
}

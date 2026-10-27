package stream

import (
	"NanoKVM-Server/common"
	"sync"
	"sync/atomic"
	"time"
)

type H264Frame struct {
	Data       []byte
	Result     int
	Duration   time.Duration
	Timestamp  int64
	HasPTS     bool
	ColorSpace [4]byte
}

type H264Subscription struct {
	frames             chan H264Frame
	done               chan struct{}
	closed             atomic.Bool
	once               sync.Once
	waitingForKeyframe bool
	source             *H264Source
}

type H264Source struct {
	mutex       sync.Mutex
	subscribers map[*H264Subscription]struct{}
	running     bool
	h265        bool
}

var defaultH264Source = &H264Source{
	subscribers: make(map[*H264Subscription]struct{}),
}

var defaultH265Source = &H264Source{subscribers: make(map[*H264Subscription]struct{}), h265: true}

func SubscribeH265() *H264Subscription { return defaultH265Source.subscribe() }

func SubscribeH264() *H264Subscription {
	return defaultH264Source.subscribe()
}

func (s *H264Source) subscribe() *H264Subscription {
	subscription := &H264Subscription{
		frames:             make(chan H264Frame, 4),
		done:               make(chan struct{}),
		source:             s,
		waitingForKeyframe: true,
	}

	s.mutex.Lock()
	SelectVideo(s.h265)
	RequestKeyframe()
	s.subscribers[subscription] = struct{}{}
	start := !s.running
	if start {
		s.running = true
	}
	s.mutex.Unlock()

	if start {
		go s.run()
	}

	return subscription
}

func (s *H264Subscription) Next() (H264Frame, bool) {
	select {
	case frame := <-s.frames:
		return frame, true
	case <-s.done:
		return H264Frame{}, false
	}
}

func (s *H264Subscription) Close() {
	s.once.Do(func() {
		s.closed.Store(true)
		close(s.done)
		s.source.remove(s)
	})
}

func (s *H264Source) remove(subscription *H264Subscription) {
	s.mutex.Lock()
	delete(s.subscribers, subscription)
	s.mutex.Unlock()
}

func (s *H264Source) run() {
	screen := common.GetScreen()
	common.CheckScreen()
	fps := screen.FPS
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	vision := common.GetKvmVision()
	startTime := time.Now()
	var clock captureClock
	idle := true

	for {
		if !capturePaced() || idle {
			<-ticker.C
		}
		idle = true
		subscribers := s.snapshot()
		if len(subscribers) == 0 {
			if s.stopIfIdle() {
				return
			}
			continue
		}

		if screen.FPS != fps && screen.FPS != 0 {
			fps = screen.FPS
			ticker.Reset(time.Second / time.Duration(fps))
		}

		data, result, pts, hasPTS, color := readVideo(vision, screen, s.h265)
		if result < 0 {
			frame := H264Frame{Result: result}
			for _, subscription := range subscribers {
				subscription.send(frame)
			}
			continue
		}
		if len(data) == 0 {
			continue
		}
		idle = false

		frame := H264Frame{
			Data:       data,
			Result:     result,
			Duration:   time.Second / time.Duration(fps),
			Timestamp:  time.Since(startTime).Microseconds(),
			HasPTS:     hasPTS,
			ColorSpace: color,
		}
		if hasPTS {
			frame.Timestamp = clock.at(pts).Microseconds()
		}
		for _, subscription := range subscribers {
			if !subscription.send(frame) {
				RequestKeyframe()
			}
		}
		GetFrameRateCounter().Update()
	}
}

// Serialize shutdown with subscribe: a replacement viewer either prevents the
// stop or starts a fresh source after the previous capture has fully released.
func (s *H264Source) stopIfIdle() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if len(s.subscribers) != 0 {
		return false
	}
	stopVideo(s.h265)
	s.running = false
	return true
}

func (s *H264Source) snapshot() []*H264Subscription {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	subscribers := make([]*H264Subscription, 0, len(s.subscribers))
	for subscription := range s.subscribers {
		subscribers = append(subscribers, subscription)
	}
	return subscribers
}

func (s *H264Subscription) send(frame H264Frame) bool {
	if s.closed.Load() {
		return false
	}
	if s.waitingForKeyframe && frame.Result >= 0 {
		if !frame.IsKeyframe() {
			return false
		}
		s.waitingForKeyframe = false
	}

	select {
	case s.frames <- frame:
		return true
	case <-s.done:
		return false
	default:
	}

	for {
		select {
		case <-s.frames:
		case <-s.done:
			return false
		default:
			s.waitingForKeyframe = true
			if !frame.IsKeyframe() && frame.Result >= 0 {
				return false
			}
			if frame.IsKeyframe() {
				s.waitingForKeyframe = false
			}
			select {
			case s.frames <- frame:
				return true
			case <-s.done:
				return false
			}
		}
	}
}

func (f H264Frame) IsKeyframe() bool { return f.Result == 3 || f.Result == 7 }

// Restarted capture pipelines reset PTS. Preserve a monotonic media timeline
// for connected decoders and Direct flow-control acknowledgements.
type captureClock struct {
	started                     bool
	base, offset, last, lastPTS time.Duration
}

func (c *captureClock) at(pts time.Duration) time.Duration {
	if !c.started || pts < c.lastPTS {
		if c.started {
			c.offset = c.last + time.Second/60
		}
		c.base, c.started = pts, true
	}
	c.lastPTS = pts
	c.last = c.offset + pts - c.base
	return c.last
}

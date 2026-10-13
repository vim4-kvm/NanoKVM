//go:build linux && vim4

package common

import (
	"NanoKVM-Server/internal/gstcapture"
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var hdmiAudio struct {
	sync.Mutex
	session    *captureSession
	retryAfter time.Time
}

func (k *KvmVision) ReadAudio() ([]byte, int) {
	hdmiAudio.Lock()
	defer hdmiAudio.Unlock()
	if hdmiAudio.session == nil {
		if time.Now().Before(hdmiAudio.retryAfter) {
			return nil, -1
		}
		ctx, cancel := context.WithCancel(context.Background())
		s := &captureSession{cancel: cancel, done: make(chan struct{}), frames: make(chan gstcapture.Frame, 2)}
		hdmiAudio.session = s
		go func() {
			defer close(s.done)
			defer close(s.frames)
			err := gstcapture.Run(ctx, "alsasrc device=hw:AMLAUGESOUND,0 buffer-time=40000 latency-time=10000 ! audio/x-raw,format=S16LE,rate=48000,channels=2 ! audioconvert ! opusenc frame-size=20 bitrate=96000 ! appsink name=frames sync=false", nil, func(f gstcapture.Frame) error {
				select {
				case s.frames <- f:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Errorf("HDMI audio: %v", err)
			}
		}()
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case f, ok := <-hdmiAudio.session.frames:
		if ok {
			return f.Data, 0
		}
		hdmiAudio.session.cancel()
		<-hdmiAudio.session.done
		hdmiAudio.session = nil
		hdmiAudio.retryAfter = time.Now().Add(time.Second)
	case <-timer.C:
	}
	return nil, -1
}

func (k *KvmVision) StopAudio() {
	hdmiAudio.Lock()
	defer hdmiAudio.Unlock()
	if s := hdmiAudio.session; s != nil {
		s.cancel()
		<-s.done
		hdmiAudio.session = nil
	}
}

//go:build linux && vim4

package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"NanoKVM-Server/internal/gstcapture"
	log "github.com/sirupsen/logrus"
)

const (
	RATE_CONTROL_CBR uint8 = iota
	RATE_CONTROL_VBR
)
const (
	IMG_MJPEG_TYPE uint8 = iota
	IMG_H264_TYPE_SPS
	IMG_H264_TYPE_PPS
	IMG_H264_TYPE_IF
	IMG_H264_TYPE_PF
	IMG_H265_TYPE_SPS
	IMG_H265_TYPE_PPS
	IMG_H265_TYPE_IF
	IMG_H265_TYPE_PF
)
const (
	STREAM_TYPE_MJPEG = iota
	STREAM_TYPE_H264_WEBRTC
	STREAM_TYPE_H264_DIRECT
	STREAM_TYPE_H265_WEBRTC
	STREAM_TYPE_H265_DIRECT
)

type videoCodec uint8

const (
	codecJPEG videoCodec = iota
	codecH264
	codecH265
)

// WebRTC and Direct transports of one codec share the same encoder pipeline.
func streamCodec(mode uint8) videoCodec {
	switch mode {
	case STREAM_TYPE_MJPEG:
		return codecJPEG
	case STREAM_TYPE_H265_WEBRTC, STREAM_TYPE_H265_DIRECT:
		return codecH265
	}
	return codecH264
}

type captureSession struct {
	cancel context.CancelFunc
	done   chan struct{}
	frames chan gstcapture.Frame
}

type KvmVision struct {
	StreamType uint8
	mu         sync.Mutex
	session    *captureSession
	dimensions atomic.Uint32
	actualFPS  atomic.Int32
	retryAfter time.Time
	settings   VideoSettings
	// Keyframe requests bypass mu so RTCP handlers never wait behind a frame read.
	keyframes chan struct{}
	// selected mirrors StreamType for readers that must not wait behind a
	// frame read holding mu (up to 500ms), such as the 20ms audio sender.
	selected atomic.Uint32
	// snapshotKey lets Snapshot read the running session without consuming it.
	snapshotKey atomic.Pointer[snapshotKey]
}

var vim4Vision = newKvmVision()

func newKvmVision() *KvmVision {
	k := &KvmVision{StreamType: STREAM_TYPE_H264_WEBRTC, settings: loadVideoSettings(), keyframes: make(chan struct{}, 1)}
	k.selected.Store(uint32(k.StreamType))
	return k
}

func GetKvmVision() *KvmVision { return vim4Vision }

func (k *KvmVision) startLocked() {
	ctx, cancel := context.WithCancel(context.Background())
	s := &captureSession{cancel: cancel, done: make(chan struct{}), frames: make(chan gstcapture.Frame, 2)}
	k.session = s
	rate := encoderFrameRate(k.settings.FPS)
	pipeline := k.settings.pipelineAtRate(k.StreamType, rate)
	// A new encoder starts with an IDR frame; discard requests made for the old one.
	select {
	case <-k.keyframes:
	default:
	}
	var keyframes <-chan struct{}
	if k.StreamType != STREAM_TYPE_MJPEG {
		keyframes = k.keyframes
		go watchEncoderRate(ctx, rate, k.settings.FPS, cancel)
	}
	codec := streamCodec(k.StreamType)
	go func() {
		defer cancel()
		defer close(s.done)
		defer close(s.frames)
		err := gstcapture.Run(ctx, pipeline, keyframes, func(frame gstcapture.Frame) error {
			k.dimensions.Store(uint32(frame.Width)<<16 | uint32(frame.Height))
			if frame.Keyframe || codec == codecJPEG {
				k.snapshotKey.Store(&snapshotKey{frame: frame, codec: codec, at: time.Now()})
			}
			// Preserve encoded prediction chains; backpressure instead of dropping frames.
			select {
			case s.frames <- frame:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Errorf("VIM4 capture: %v", err)
		}
	}()
}

// ReadH264Sample returns an access unit and its capture timestamp.
func (k *KvmVision) ReadH264Sample(_, _, _ uint16) ([]byte, int, time.Duration, bool) {
	f, result := k.readSample(codecH264)
	return f.Data, result, f.PTS, result >= 0 && f.HasPTS
}

// ReadH265Sample returns an access unit and its capture timestamp.
func (k *KvmVision) ReadH265Sample(_, _, _ uint16) ([]byte, int, time.Duration, bool) {
	f, result := k.readSample(codecH265)
	return f.Data, result, f.PTS, result >= 0 && f.HasPTS
}

// RequestKeyframe asks the running H.264/H.265 encoder for an IDR frame.
func (k *KvmVision) RequestKeyframe() {
	select {
	case k.keyframes <- struct{}{}:
	default:
	}
}

func (k *KvmVision) readSample(codec videoCodec) (gstcapture.Frame, int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if codec != streamCodec(k.StreamType) {
		return gstcapture.Frame{}, -1
	}
	if k.session == nil {
		if time.Now().Before(k.retryAfter) {
			return gstcapture.Frame{}, -1
		}
		k.startLocked()
	}
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	select {
	case f, ok := <-k.session.frames:
		if !ok {
			k.session.cancel()
			<-k.session.done
			k.session = nil
			k.retryAfter = time.Now().Add(time.Second)
			return gstcapture.Frame{}, -2
		}
		if !f.CopiedAt.IsZero() {
			ageMS := -1.0
			if f.CaptureAge >= 0 {
				ageMS = float64(f.CaptureAge) / float64(time.Millisecond)
			}
			log.Infof("VIM4 timing read pts_ns=%d capture_to_copy_ms=%.3f copy_to_read_ms=%.3f bytes=%d key=%t",
				f.PTS, ageMS, float64(time.Since(f.CopiedAt))/float64(time.Millisecond), len(f.Data), f.Keyframe)
		}
		return f, sampleResult(codec, f.Keyframe)
	case <-timer.C:
		return gstcapture.Frame{}, -1
	}
}

// Direct streams mark keyframes from the libkvm frame-type result.
func sampleResult(codec videoCodec, keyframe bool) int {
	switch {
	case codec == codecJPEG:
		return int(IMG_MJPEG_TYPE)
	case codec == codecH265 && keyframe:
		return int(IMG_H265_TYPE_IF)
	case codec == codecH265:
		return int(IMG_H265_TYPE_PF)
	case keyframe:
		return int(IMG_H264_TYPE_IF)
	}
	return int(IMG_H264_TYPE_PF)
}

func (k *KvmVision) ReadH264(w, h, rate uint16) ([]byte, int) {
	d, result, _, _ := k.ReadH264Sample(w, h, rate)
	return d, result
}
func (k *KvmVision) ReadH265(w, h, rate uint16) ([]byte, int) {
	d, result, _, _ := k.ReadH265Sample(w, h, rate)
	return d, result
}
func (k *KvmVision) StopVideo() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stopLocked()
}

// WithVideoPaused keeps frame readers and snapshots from reopening capture
// until the receiver has applied a board-level HDMI change.
func (k *KvmVision) WithVideoPaused(action func() error) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stopLocked()
	return action()
}
func (k *KvmVision) stopLocked() {
	if k.session != nil {
		k.session.cancel()
		<-k.session.done
		k.session = nil
	}
}
func (k *KvmVision) Dimensions() (uint16, uint16) {
	d := k.dimensions.Load()
	return uint16(d >> 16), uint16(d)
}
func (k *KvmVision) Close() { k.StopVideo() }

func (k *KvmVision) GetStreamType() uint8 {
	return uint8(k.selected.Load())
}

// A previous transport's teardown must not close the new transport's encoder.
func (k *KvmVision) StopVideoFor(mode uint8) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.StreamType == mode {
		k.stopLocked()
	}
}
func (k *KvmVision) SetStreamType(mode uint8) {
	if mode > STREAM_TYPE_H265_DIRECT {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.StreamType != mode {
		if streamCodec(k.StreamType) != streamCodec(mode) {
			k.stopLocked()
		}
		k.StreamType = mode
		k.selected.Store(uint32(mode))
		k.retryAfter = time.Time{}
	}
}
func (k *KvmVision) SetRateControl(mode uint8) int {
	if mode != RATE_CONTROL_CBR && mode != RATE_CONTROL_VBR {
		return -1
	}
	return k.updateControl(func(s *VideoSettings) {
		s.RateControl = "cbr"
		if mode == RATE_CONTROL_VBR {
			s.RateControl = "vbr"
		}
	})
}
func (k *KvmVision) ReadMjpeg(uint16, uint16, uint16) ([]byte, int) {
	f, result := k.readSample(codecJPEG)
	return f.Data, result
}
func (k *KvmVision) SetFps(fps uint8) int {
	return k.updateControl(func(s *VideoSettings) { s.FPS = int(fps) })
}
func (k *KvmVision) GetFps() int            { return int(k.actualFPS.Load()) }
func (k *KvmVision) SetActualFPS(fps int32) { k.actualFPS.Store(fps) }
func (k *KvmVision) SetHDMI(bool) int       { return -1 }
func (k *KvmVision) SetGop(gop uint8) int {
	return k.updateControl(func(s *VideoSettings) { s.GOP = int(gop) })
}

func (k *KvmVision) updateControl(update func(*VideoSettings)) int {
	if err := k.UpdateVideoSettings(update); err != nil {
		log.Warnf("update video control: %v", err)
		return -1
	}
	return 0
}

func (k *KvmVision) SetFrameDetect(uint8) {}

func (k *KvmVision) HasHDMISignal() bool {
	return knownHDMIFrameRate(hdmiInfo(), 0) > 0
}

// vbrQPMax caps quantization in VBR mode. With moving 4K desktop content and a
// 3000 Kbps target, the vendor control averaged 4.0 Mbps; a ceiling of 38
// raised it to 8.6 Mbps and 34 to 11.9 Mbps instead of degrading the picture.
const vbrQPMax = 36

func (s VideoSettings) pipeline(mode uint8) string {
	return s.pipelineAtRate(mode, encoderFrameRate(s.FPS))
}

func (s VideoSettings) pipelineAtRate(mode uint8, rate int) string {
	p := captureFactory()
	if wave5Backend() {
		p += fmt.Sprintf(" height=%d", s.Height)
	}
	p += " ! queue max-size-buffers=3 max-size-bytes=0 max-size-time=0 ! "
	_, _, hardwareJPEG := gstcapture.IntPropertyRange("vim4jpegenc", "quality")
	fps := s.FPS
	if mode == STREAM_TYPE_MJPEG && !hardwareJPEG && (fps == 0 || fps > 10) {
		fps = 10
	}
	if fps > 0 {
		p += fmt.Sprintf("videorate drop-only=true ! video/x-raw,framerate=%d/1 ! ", fps)
	}
	if mode == STREAM_TYPE_MJPEG {
		if hardwareJPEG {
			p += fmt.Sprintf("vim4jpegenc quality=%d ! ", s.Quality)
		} else {
			p += fmt.Sprintf("videoconvert ! video/x-raw,format=I420 ! jpegenc quality=%d ! ", s.Quality)
		}
	} else {
		// Coalesce PLIs from several viewers into one forced IDR.
		budget := ""
		if min, max, ok := gstcapture.IntPropertyRange(encoderFactory(), "framerate"); ok && rate >= min && rate <= max {
			budget = fmt.Sprintf(" framerate=%d", rate)
		} else {
			log.Warnf("amlvenc cannot set %d fps budget; using plugin default (bitrate may differ)", rate)
		}
		if gop, ok := encoderExperiment("gop", "VIM4_ENCODER_GOP", 1); ok {
			s.GOP = gop
		}
		budget += encoderExperimentOptions()
		if s.RateControl == "vbr" {
			if _, _, ok := gstcapture.IntPropertyRange(encoderFactory(), "qp-max"); ok {
				budget += fmt.Sprintf(" qp-max=%d", vbrQPMax)
			} else {
				log.Warn("amlvenc has no qp-max; VBR falls back to target-bitrate control")
			}
		}
		codec := "h264"
		if streamCodec(mode) == codecH265 {
			codec = "h265"
		}
		p += fmt.Sprintf("%s bitrate=%d gop=%d%s min-force-key-unit-interval=250000000 ! video/x-%s ! %sparse config-interval=-1 ! video/x-%s,stream-format=byte-stream,alignment=au ! ", encoderFactory(), s.Bitrate, s.GOP, budget, codec, codec, codec)
	}
	return p + "appsink name=frames sync=false"
}

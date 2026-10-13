//go:build linux && vim4

package common

import (
	"NanoKVM-Server/internal/gstcapture"
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVideoSettingsRejectUnsupportedControls(t *testing.T) {
	valid := VideoSettings{1920, 1080, 8000, 30, 60, 80, "cbr"}
	for _, change := range []func(*VideoSettings){
		func(s *VideoSettings) { s.Height = 720 }, func(s *VideoSettings) { s.Bitrate = 20000 },
		func(s *VideoSettings) { s.FPS = 61 }, func(s *VideoSettings) { s.Quality = 0 }, func(s *VideoSettings) { s.GOP = 0 },
		func(s *VideoSettings) { s.RateControl = "" }, func(s *VideoSettings) { s.RateControl = "CBR" },
	} {
		s := valid
		change(&s)
		if s.Validate() == nil {
			t.Fatalf("accepted unsupported controls: %+v", s)
		}
	}
}

func TestSettingsWithoutRateControlKeepStoredValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(`{"width":1920,"height":1080,"bitrate":5000,"gop":60,"fps":30,"quality":70}`), 0600); err != nil {
		t.Fatal(err)
	}
	previous := videoSettingsPath
	videoSettingsPath = path
	defer func() { videoSettingsPath = previous }()
	if got := loadVideoSettings(); got != (VideoSettings{1920, 1080, 5000, 60, 30, 70, "cbr"}) {
		t.Fatalf("upgrade changed stored settings: %+v", got)
	}
}

func TestJPEGProducesDecodableFrames(t *testing.T) {
	s := VideoSettings{1280, 720, 8000, 30, 0, 80, "cbr"}
	pipeline := strings.Replace(s.pipeline(STREAM_TYPE_MJPEG), "vfmcapsrc", "videotestsrc num-buffers=12 ! video/x-raw,width=1280,height=720,framerate=60/1", 1)
	frames := 0
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := gstcapture.Run(ctx, pipeline, nil, func(f gstcapture.Frame) error {
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(f.Data))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != 1280 || cfg.Height != 720 {
			t.Fatalf("wrong dimensions: %+v", cfg)
		}
		frames++
		return nil
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("pipeline did not finish: %v", err)
	}
	if frames < 1 {
		t.Fatal("no JPEG frames")
	}
}

func TestReadersOnlyConsumeTheirCodec(t *testing.T) {
	k := KvmVision{StreamType: STREAM_TYPE_H265_DIRECT, retryAfter: time.Now().Add(time.Hour)}
	for name, read := range map[string]func() int{
		"h264":  func() int { _, r := k.ReadH264(0, 0, 0); return r },
		"mjpeg": func() int { _, r := k.ReadMjpeg(0, 0, 0); return r },
	} {
		if r := read(); r != -1 || k.session != nil {
			t.Fatalf("%s reader used the H.265 session: result %d", name, r)
		}
	}
	if !strings.Contains(VideoSettings{1920, 1080, 8000, 30, 60, 80, "cbr"}.pipeline(STREAM_TYPE_H265_WEBRTC), "video/x-h265 ! h265parse") {
		t.Fatal("H.265 transport did not request H.265 from amlvenc")
	}
}

func TestDirectKeyframeResults(t *testing.T) {
	for _, c := range []struct {
		codec    videoCodec
		keyframe bool
		want     uint8
	}{
		{codecH264, true, IMG_H264_TYPE_IF}, {codecH264, false, IMG_H264_TYPE_PF},
		{codecH265, true, IMG_H265_TYPE_IF}, {codecH265, false, IMG_H265_TYPE_PF},
		{codecJPEG, true, IMG_MJPEG_TYPE},
	} {
		if got := sampleResult(c.codec, c.keyframe); got != int(c.want) {
			t.Errorf("codec %d keyframe %t: got %d, want %d", c.codec, c.keyframe, got, c.want)
		}
	}
}

// Audio and transport loops poll the selected type every 20ms; a frame read
// holds mu for up to 500ms while waiting for the encoder.
func TestStreamTypeDoesNotWaitForFrameRead(t *testing.T) {
	k := newKvmVision()
	k.SetStreamType(STREAM_TYPE_MJPEG)
	k.mu.Lock()
	defer k.mu.Unlock()
	done := make(chan uint8)
	go func() { done <- k.GetStreamType() }()
	select {
	case got := <-done:
		if got != STREAM_TYPE_MJPEG {
			t.Fatalf("got stream type %d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("GetStreamType blocked behind the video lock")
	}
}

func TestOldTransportCannotStopNewSession(t *testing.T) {
	canceled := false
	done := make(chan struct{})
	close(done)
	k := KvmVision{StreamType: STREAM_TYPE_MJPEG, session: &captureSession{cancel: func() { canceled = true }, done: done}}
	k.StopVideoFor(STREAM_TYPE_H264_WEBRTC)
	if canceled || k.session == nil {
		t.Fatal("old transport closed the new session")
	}
	k.StopVideoFor(STREAM_TYPE_MJPEG)
	if !canceled || k.session != nil {
		t.Fatal("active transport did not release session")
	}
}

func TestSameCodecTransportSwitchKeepsSessionAndOldStopCannotKillIt(t *testing.T) {
	for _, modes := range [][2]uint8{{STREAM_TYPE_H264_WEBRTC, STREAM_TYPE_H264_DIRECT}, {STREAM_TYPE_H265_WEBRTC, STREAM_TYPE_H265_DIRECT}} {
		k := newKvmVision()
		k.SetStreamType(modes[0])
		done := make(chan struct{})
		close(done)
		canceled := false
		session := &captureSession{cancel: func() { canceled = true }, done: done}
		k.session = session
		k.SetStreamType(modes[1])
		if canceled || k.session != session {
			t.Fatal("same codec switch restarted encoder")
		}
		k.StopVideoFor(modes[0])
		if canceled || k.session != session {
			t.Fatal("old transport stopped replacement")
		}
		k.StopVideoFor(modes[1])
		if !canceled || k.session != nil {
			t.Fatal("active transport failed to release encoder")
		}
	}
}

func TestSnapshotScalesToJPEG(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := snapshotJPEG(ctx, "videotestsrc num-buffers=1 ! video/x-raw,width=3840,height=2160 ! videoconvert", 3840, 2160)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 1920 || cfg.Height != 1080 {
		t.Fatalf("snapshot %dx%d: %v", cfg.Width, cfg.Height, err)
	}
}

func TestSnapshotWaitsForFreshFrameFromSession(t *testing.T) {
	k := newKvmVision()
	k.SetStreamType(STREAM_TYPE_MJPEG)
	k.session = &captureSession{}
	k.snapshotKey.Store(&snapshotKey{frame: gstcapture.Frame{Data: []byte("old")}, codec: codecJPEG, at: time.Now().Add(-time.Second)})
	go func() {
		time.Sleep(50 * time.Millisecond)
		k.snapshotKey.Store(&snapshotKey{frame: gstcapture.Frame{Data: []byte("new")}, codec: codecJPEG, at: time.Now()})
	}()
	data, err := k.Snapshot(context.Background())
	if err != nil || string(data) != "new" {
		t.Fatalf("got %q, %v", data, err)
	}
	if k.GetStreamType() != STREAM_TYPE_MJPEG {
		t.Fatal("snapshot changed the selected transport")
	}
}

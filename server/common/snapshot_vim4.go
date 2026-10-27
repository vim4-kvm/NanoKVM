//go:build linux && vim4

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"time"

	"NanoKVM-Server/internal/gstcapture"
)

// SnapshotImage applies API dimensions and quality after obtaining a fresh,
// independent frame. It never consumes the streaming encoder's frame queue.
func (k *KvmVision) SnapshotImage(ctx context.Context, width, height, quality uint16) ([]byte, error) {
	data, err := k.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := int(width), int(height)
	if w == 0 && h == 0 {
		w, h = cfg.Width, cfg.Height
	} else if w == 0 {
		w = cfg.Width * h / cfg.Height
	} else if h == 0 {
		h = cfg.Height * w / cfg.Width
	}
	if w < 1 || h < 1 || w > 3840 || h > 2160 || quality > 100 {
		return nil, errors.New("invalid screenshot dimensions or quality")
	}
	if quality == 0 {
		quality = 80
	}
	file, err := os.CreateTemp("", "vim4-jpeg-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	pipeline := fmt.Sprintf("filesrc location=%s ! jpegdec ! videoconvert ! videoscale ! video/x-raw,format=I420,width=%d,height=%d ! jpegenc quality=%d ! appsink name=frames sync=false", file.Name(), w, h, quality)
	var output []byte
	err = gstcapture.Run(ctx, pipeline, nil, func(f gstcapture.Frame) error { output = f.Data; return io.EOF })
	if len(output) > 0 {
		return output, nil
	}
	return nil, fmt.Errorf("encode screenshot: %w", err)
}

// snapshotKey is the newest independently decodable frame of the running session.
type snapshotKey struct {
	frame gstcapture.Frame
	codec videoCodec
	at    time.Time
}

const snapshotMaxWidth = 1920

// Snapshot returns one JPEG of the HDMI input without changing the selected
// transport: vfm_cap has a single consumer, so an MJPEG request would stop a
// viewer's H.264/H.265 encoder. A running encoder is asked for an IDR, which
// is decoded in software; without a session the input is captured directly.
func (k *KvmVision) Snapshot(ctx context.Context) ([]byte, error) {
	k.mu.Lock()
	if k.session == nil {
		defer k.mu.Unlock()
		return snapshotJPEG(ctx, captureFactory()+" num-buffers=1 ! videoconvert", k.settings.Width, k.settings.Height)
	}
	codec := streamCodec(k.StreamType)
	k.mu.Unlock()

	requested := time.Now()
	if codec != codecJPEG {
		k.RequestKeyframe()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if key := k.snapshotKey.Load(); key != nil && key.at.After(requested) {
			if key.frame.ColorSpace[1] == 16 && H265Main10() {
				// JPEG consumers need the hardware-mapped VDIN1 picture. An
				// eight-bit conversion of the PQ keyframe is not tone mapping.
				return snapshotJPEG(ctx, captureFactory()+" num-buffers=1 ! videoconvert", key.frame.Width, key.frame.Height)
			}
			return decodeSnapshot(ctx, key)
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("no frame from the running session")
		case <-ticker.C:
		}
	}
}

func decodeSnapshot(ctx context.Context, key *snapshotKey) ([]byte, error) {
	if key.codec == codecJPEG {
		return key.frame.Data, nil
	}
	file, err := os.CreateTemp("", "vim4-snapshot-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(key.frame.Data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	name := "h264"
	if key.codec == codecH265 {
		name = "h265"
	}
	source := fmt.Sprintf("filesrc location=%s ! %sparse ! avdec_%s ! videoconvert", file.Name(), name, name)
	return snapshotJPEG(ctx, source, key.frame.Width, key.frame.Height)
}

// snapshotJPEG encodes the first frame of source, scaled down to at most
// snapshotMaxWidth; screenshots for remote tools do not need 4K.
func snapshotJPEG(ctx context.Context, source string, width, height int) ([]byte, error) {
	if width > snapshotMaxWidth && height > 0 {
		height = height * snapshotMaxWidth / width
		width = snapshotMaxWidth
	}
	caps := "video/x-raw,format=I420"
	if width > 0 && height > 0 {
		caps += fmt.Sprintf(",width=%d,height=%d", width, height)
	}
	pipeline := source + " ! videoscale ! " + caps + " ! jpegenc quality=90 ! appsink name=frames sync=false"
	var jpeg []byte
	err := gstcapture.Run(ctx, pipeline, nil, func(f gstcapture.Frame) error {
		jpeg = f.Data
		return io.EOF
	})
	if jpeg == nil {
		if err == nil || errors.Is(err, io.EOF) {
			err = errors.New("snapshot produced no frame")
		}
		return nil, err
	}
	return jpeg, nil
}

//go:build linux && vim4

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var videoSettingsPath = "/etc/kvm/video.json"

type VideoSettings struct {
	Width   int `json:"width"`
	Height  int `json:"height"`
	Bitrate int `json:"bitrate"`
	GOP     int `json:"gop"`
	FPS     int `json:"fps"`
	Quality int `json:"quality"`
	// cbr: vendor target-bitrate control; quality falls in motion.
	// vbr: adds a QP ceiling so quality holds and bitrate may exceed the target.
	RateControl string `json:"rateControl"`
}

func (s VideoSettings) Validate() error {
	if !((s.Width == 1280 && s.Height == 720) || (s.Width == 1920 && s.Height == 1080) || (s.Width == 3840 && s.Height == 2160)) {
		return fmt.Errorf("unsupported capture resolution")
	}
	if s.Bitrate < 1000 || s.Bitrate > 60000 || s.GOP < 1 || s.GOP > 200 || s.FPS < 0 || s.FPS > 60 || s.Quality < 1 || s.Quality > 100 {
		return fmt.Errorf("bitrate must be 1000–60000, GOP 1–200, FPS 0–60 and JPEG quality 1–100")
	}
	if s.RateControl != "cbr" && s.RateControl != "vbr" {
		return fmt.Errorf("rate control must be cbr or vbr")
	}
	return nil
}

func loadVideoSettings() VideoSettings {
	defaults := VideoSettings{3840, 2160, 12000, 120, 0, 80, "cbr"}
	data, err := os.ReadFile(videoSettingsPath)
	// Fields absent from older files keep their defaults.
	s := defaults
	if err != nil || json.Unmarshal(data, &s) != nil || s.Validate() != nil {
		return defaults
	}
	return s
}

func (k *KvmVision) VideoSettings() VideoSettings {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.settings
}

// Keep read-modify-write operations atomic across simultaneous control requests.
// Only the root-owned helper may restart the HDMI service or write VDIN sysfs.
func (k *KvmVision) UpdateVideoSettings(update func(*VideoSettings)) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	s := k.settings
	update(&s)
	if err := s.Validate(); err != nil {
		return err
	}
	if s == k.settings {
		return nil
	}
	k.stopLocked()
	if s.Width != k.settings.Width || s.Height != k.settings.Height {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "sudo", "-n", "/usr/libexec/vim4-video", strconv.Itoa(s.Width), strconv.Itoa(s.Height)).CombinedOutput(); err != nil {
			return fmt.Errorf("configure HDMI: %w: %s", err, out)
		}
	}
	k.settings = s
	k.retryAfter = time.Time{}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(videoSettingsPath+".new", data, 0600); err != nil {
		return err
	}
	return os.Rename(videoSettingsPath+".new", videoSettingsPath)
}

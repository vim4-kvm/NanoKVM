//go:build linux && vim4

package common

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"NanoKVM-Server/internal/gstcapture"
	log "github.com/sirupsen/logrus"
)

// vfm_cap currently advertises an unspecified frame rate. The HDMI receiver
// reports hundredths of a frame/second; use it until capture exposes this in
// negotiated caps. A requested FPS is a drop-only ceiling, never an upsample.
func encoderFrameRate(limit int) int {
	info, _ := os.ReadFile("/sys/class/hdmirx/hdmirx0/info")
	return hdmiEncoderFrameRate(string(info), limit)
}

func hdmiEncoderFrameRate(info string, limit int) int {
	rate := knownHDMIFrameRate(info, limit)
	if rate == 0 {
		rate = 30
		if limit > 0 && limit < rate {
			rate = limit
		}
	}
	return rate
}

func knownHDMIFrameRate(info string, limit int) int {
	rate := 0
	for _, line := range strings.Split(info, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "Frame Rate" {
			continue
		}
		hundredths, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && hundredths >= 100 && hundredths <= 12000 {
			rate = (hundredths + 50) / 100
		}
		break
	}
	if limit > 0 && limit < rate {
		rate = limit
	}
	return rate
}

// Ignore loss of lock and debounce valid changes before reopening the codec.
// The budget is immutable for a running vendor encoder.
func watchEncoderRate(ctx context.Context, initial, limit int, changed context.CancelFunc) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var change rateChange
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, _ := os.ReadFile("/sys/class/hdmirx/hdmirx0/info")
			rate := knownHDMIFrameRate(string(info), limit)
			if change.observe(initial, rate) {
				log.Infof("VIM4 HDMI budget changed from %d to %d fps; reopening capture", initial, rate)
				changed()
				return
			}
		}
	}
}

type rateChange struct{ candidate, count int }

func (c *rateChange) observe(initial, rate int) bool {
	if rate == 0 || rate == initial {
		c.candidate, c.count = 0, 0
		return false
	}
	if rate != c.candidate {
		c.candidate, c.count = rate, 0
	}
	c.count++
	return c.count >= 3
}

// Experiments are operator-only service overrides, never UI defaults. Validate
// against the loaded plugin so an old encoder remains usable after rollback.
func encoderExperiment(property, environment string, minimum int) (int, bool) {
	raw := os.Getenv(environment)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	low, high, supported := gstcapture.IntPropertyRange("amlvenc", property)
	if err != nil || !supported || value < minimum || value < low || value > high {
		log.Warnf("Ignoring invalid or unsupported %s", environment)
		return 0, false
	}
	log.Warnf("Experimental encoder setting: %s=%d", property, value)
	return value, true
}

func encoderExperimentOptions() string {
	options := ""
	for _, option := range []struct{ property, environment string }{
		{"i-qp-min", "VIM4_ENCODER_I_QP_MIN"},
		{"intra-refresh-rows", "VIM4_ENCODER_INTRA_REFRESH_ROWS"},
	} {
		if value, ok := encoderExperiment(option.property, option.environment, 0); ok {
			options += fmt.Sprintf(" %s=%d", option.property, value)
		}
	}
	return options
}

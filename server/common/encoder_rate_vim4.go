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
	return hdmiEncoderFrameRate(hdmiInfo(), limit)
}

func wave5Backend() bool { return os.Getenv("VIM4_VIDEO_BACKEND") == "wave5" }

func secondaryHDMICapture() bool {
	return !wave5Backend() && os.Getenv("VIM4_SECONDARY_HDMI_CAPTURE") == "1"
}

// H.265 reserves a Main10-capable receiver for the lifetime of the session:
// the global mapping and source EOTF may change without a new SDP exchange.
// Eight-bit Main SDR also conforms to Main10, so the same track can carry both.
func H265Main10() bool {
	if !secondaryHDMICapture() {
		return false
	}
	capable, ok := gstcapture.IntPropertyValue("vim4h265src", "hdr-capable")
	return ok && capable == 1
}

func captureFactory() string {
	if wave5Backend() {
		if _, broker := os.LookupEnv("VIM4_BROKER_SOCKET"); broker {
			return "vim4brokersrc"
		}
		return "vim4v4l2src"
	}
	if secondaryHDMICapture() {
		return "vim4vdin1src"
	}
	return "vfmcapsrc"
}

func encoderFactory() string {
	if wave5Backend() {
		return "vim4wave5enc"
	}
	return "amlvenc"
}

func hdmiInfo() string {
	if wave5Backend() {
		rate, ok := gstcapture.IntPropertyValue(captureFactory(), "input-rate")
		if !ok {
			return ""
		}
		return fmt.Sprintf("Frame Rate: %d\n", rate)
	}
	if !vendorSignalReady() {
		return ""
	}
	info, _ := os.ReadFile("/sys/class/hdmirx/hdmirx0/info")
	if !vendorSignalReady() {
		return ""
	}
	return string(info)
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
	// New receivers expose a stable base budget during VRR. Older kernels
	// retain the measured-rate fallback, including the Wave5 property above.
fields:
	for _, name := range []string{"Capture Frame Rate", "Frame Rate"} {
		for _, line := range strings.Split(info, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok || strings.TrimSpace(key) != name {
				continue
			}
			hundredths, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil && hundredths >= 100 && hundredths <= 12000 {
				rate = (hundredths + 50) / 100
			}
			break fields
		}
	}
	if limit > 0 && limit < rate {
		rate = limit
	}
	return rate
}

// Watch every stream type, including JPEG. Geometry may change without a new
// frame rate or an EOS/error from the capture driver.
func watchEncoderInput(ctx context.Context, initial inputMode, changed context.CancelFunc) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var change inputChange
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			input := parseInputMode(hdmiInfo())
			if change.observe(initial, input) {
				log.Infof("VIM4 HDMI input changed from %+v to %+v; reopening capture", initial, input)
				changed()
				return
			}
		}
	}
}

// Experiments are operator-only service overrides, never UI defaults. Validate
// against the loaded plugin so an old encoder remains usable after rollback.
func encoderExperiment(property, environment string, minimum int) (int, bool) {
	raw := os.Getenv(environment)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	low, high, supported := gstcapture.IntPropertyRange(encoderFactory(), property)
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

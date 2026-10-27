//go:build linux && vim4

package common

import (
	"fmt"
	"testing"
)

func TestStaleTimingsDoNotEstablishReceiverLock(t *testing.T) {
	for _, text := range []string{"3b11", "0", "", "invalid"} {
		if signalLocked(text) {
			t.Fatalf("unlocked receiver accepted: %q", text)
		}
	}
	if !signalLocked("13b11\n") {
		t.Fatal("locked receiver rejected")
	}
}

func TestVRRCadenceKeepsEncoderBudget(t *testing.T) {
	var change inputChange
	for _, rate := range []int{4800, 6000, 9000, 9000, 9000, 11988} {
		info := fmt.Sprintf("Frame Rate: %d\nCapture Frame Rate: 12000\n", rate)
		if got := knownHDMIFrameRate(info, 0); got != 120 || change.observe(inputMode{rate: 120}, parseInputMode(info)) {
			t.Fatalf("VRR cadence %d changed capture budget to %d", rate, got)
		}
		if got := knownHDMIFrameRate(info, 60); got != 60 {
			t.Fatalf("VRR input bypassed output limit: %d", got)
		}
	}
	if knownHDMIFrameRate("Capture Frame Rate: 0\nFrame Rate: 4800", 0) != 0 {
		t.Fatal("unknown base timing fell back to variable cadence")
	}
}

func TestInputChangeRequiresMatchingObservations(t *testing.T) {
	var change inputChange
	for _, rate := range []int{60, 0, 60, 50, 30, 60} {
		if change.observe(inputMode{rate: 30}, inputMode{rate: rate}) {
			t.Fatalf("premature restart at %d", rate)
		}
	}
	if !change.observe(inputMode{rate: 30}, inputMode{rate: 60}) {
		t.Fatal("stable new signal did not restart")
	}
	if knownHDMIFrameRate("Frame Rate: 0", 15) != 0 {
		t.Fatal("unlocked source must remain unknown")
	}
	if knownHDMIFrameRate("Frame Rate: 6000", 30) != 30 {
		t.Fatal("drop ceiling ignored")
	}
}

func TestInputChangeIsIndependentOfOutputRateLimit(t *testing.T) {
	current := parseInputMode("Hactive: 3840\nVactive: 2160\nFrame Rate: 6000\nHDR EOTF: SDR")
	for _, info := range []string{
		"Hactive: 3440\nVactive: 1440\nFrame Rate: 6000\nHDR EOTF: SDR",
		"Hactive: 3840\nVactive: 2160\nFrame Rate: 6000\nHDR EOTF: SMPTE_ST_2048",
		"Hactive: 3840\nVactive: 2160\nFrame Rate: 5000\nHDR EOTF: SDR",
		"",
	} {
		var change inputChange
		next := parseInputMode(info)
		if change.observe(current, next) || !change.observe(current, next) {
			t.Fatalf("input change was missed: %q", info)
		}
	}
}

func TestHDMIEncoderBudgetMatchesSourceAndDropLimit(t *testing.T) {
	for _, tc := range []struct {
		info        string
		limit, want int
	}{
		{"Color Depth: 10\nFrame Rate: 5999\nSkip frame: 0", 0, 60},
		{"Frame Rate: 5994", 30, 30},
		{"Frame Rate: 2997", 60, 30},
		{"Frame Rate: 2398", 0, 24},
		{"Frame Rate: 5000", 0, 50},
		{"Frame Rate: 0", 0, 30},
		{"Frame Rate: invalid", 10, 10},
		{"", 0, 30},
	} {
		if got := hdmiEncoderFrameRate(tc.info, tc.limit); got != tc.want {
			t.Errorf("info=%q limit=%d: got %d, want %d", tc.info, tc.limit, got, tc.want)
		}
	}
}

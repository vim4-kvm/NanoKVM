//go:build linux && vim4

package common

import "testing"

func TestRateChangeRequiresStableSignal(t *testing.T) {
	var change rateChange
	for _, rate := range []int{60, 60, 0, 60, 50, 50, 30, 60, 60} {
		if change.observe(30, rate) {
			t.Fatalf("premature restart at %d", rate)
		}
	}
	if !change.observe(30, 60) {
		t.Fatal("stable new signal did not restart")
	}
	if knownHDMIFrameRate("Frame Rate: 0", 15) != 0 {
		t.Fatal("unlocked source must remain unknown")
	}
	if knownHDMIFrameRate("Frame Rate: 6000", 30) != 30 {
		t.Fatal("drop ceiling ignored")
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

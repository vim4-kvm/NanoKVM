package stream

import (
	"testing"
	"time"
)

func TestSubscriberOverflowWaitsForIDRAndReportsFailures(t *testing.T) {
	s := &H264Subscription{frames: make(chan H264Frame, 2), done: make(chan struct{}), waitingForKeyframe: true}
	if s.send(H264Frame{Result: 4}) {
		t.Fatal("new viewer received prediction frame")
	}
	if !s.send(H264Frame{Result: 3}) || !s.send(H264Frame{Result: 4}) {
		t.Fatal("valid chain rejected")
	}
	if s.send(H264Frame{Result: 4}) {
		t.Fatal("overflow preserved a broken chain")
	}
	if !s.send(H264Frame{Result: -2}) {
		t.Fatal("failure state hidden while waiting for IDR")
	}
	if s.send(H264Frame{Result: 4}) {
		t.Fatal("failure cleared IDR requirement")
	}
	if !s.send(H264Frame{Result: 3}) {
		t.Fatal("IDR did not recover viewer")
	}
}

func TestH265QueueRecoveryAndIndependentViewers(t *testing.T) {
	slow := &H264Subscription{frames: make(chan H264Frame, 1), done: make(chan struct{})}
	fast := &H264Subscription{frames: make(chan H264Frame, 1), done: make(chan struct{})}
	for _, s := range []*H264Subscription{slow, fast} {
		s.send(H264Frame{Result: 7})
	}
	<-fast.frames
	if slow.send(H264Frame{Result: 8}) {
		t.Fatal("slow viewer retained broken chain")
	}
	if !fast.send(H264Frame{Result: 8}) {
		t.Fatal("slow viewer blocked fast viewer")
	}
	if !slow.send(H264Frame{Result: 7}) {
		t.Fatal("H265 IDR did not recover")
	}
}

func TestCaptureClockPreservesPTSIntervalsAndRestart(t *testing.T) {
	var clock captureClock
	a := clock.at(time.Second)
	b := clock.at(time.Second + 17*time.Millisecond)
	c := clock.at(0)
	d := clock.at(33 * time.Millisecond)
	if b-a != 17*time.Millisecond || c <= b || d-c != 33*time.Millisecond {
		t.Fatalf("nonmonotonic or wall-clock timing: %v %v %v %v", a, b, c, d)
	}
}

func TestSubscriptionCloseOnlyRemovesItsSource(t *testing.T) {
	source := &H264Source{subscribers: make(map[*H264Subscription]struct{}), h265: true}
	sub := &H264Subscription{source: source, done: make(chan struct{})}
	source.subscribers[sub] = struct{}{}
	sub.Close()
	sub.Close()
	if len(source.subscribers) != 0 {
		t.Fatal("H265 viewer leaked")
	}
	if _, ok := sub.Next(); ok {
		t.Fatal("closed subscription still reading")
	}
}

func TestSourceRemainsRunningUntilLastTransportLeaves(t *testing.T) {
	for _, h265 := range []bool{false, true} {
		source := &H264Source{subscribers: make(map[*H264Subscription]struct{}), running: true, h265: h265}
		direct := source.subscribe()
		rtc := source.subscribe()
		direct.Close()
		if source.stopIfIdle() || !source.running {
			t.Fatal("Direct teardown stopped active WebRTC viewer")
		}
		replacement := source.subscribe()
		rtc.Close()
		if source.stopIfIdle() {
			t.Fatal("old WebRTC teardown stopped replacement")
		}
		replacement.Close()
		if !source.stopIfIdle() || source.running {
			t.Fatal("last viewer did not stop source")
		}
	}
}

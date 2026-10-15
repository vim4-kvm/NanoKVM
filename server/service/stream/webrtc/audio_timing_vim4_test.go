//go:build linux && vim4

package webrtc

import "testing"

func TestAudioQueueOverflowPreservesMissingMediaTime(t *testing.T) {
	queue := make(chan audioPacket, 4)
	for sequence := uint64(1); sequence <= 8; sequence++ {
		select {
		case queue <- audioPacket{sequence: sequence}:
		default:
		}
	}
	var previous uint64
	for len(queue) > 0 {
		packet := <-queue
		if got := packet.sample(previous).PrevDroppedPackets; got != 0 {
			t.Fatalf("contiguous packet reports %d drops", got)
		}
		previous = packet.sequence
	}
	packet := audioPacket{sequence: 9}
	if got := packet.sample(previous).PrevDroppedPackets; got != 4 {
		t.Fatalf("lost media intervals = %d, want 4", got)
	}
	if got := packet.sample(0).PrevDroppedPackets; got != 0 {
		t.Fatalf("new viewer reports %d historical drops", got)
	}
}

func TestAudioGapCannotWrapDropCount(t *testing.T) {
	if got := (audioPacket{sequence: 100000}).sample(1).PrevDroppedPackets; got != 65535 {
		t.Fatalf("large gap wrapped to %d", got)
	}
	if got := (audioPacket{sequence: 1}).sample(100).PrevDroppedPackets; got != 0 {
		t.Fatalf("sequence reset reports %d drops", got)
	}
}

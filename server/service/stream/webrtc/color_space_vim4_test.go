//go:build vim4

package webrtc

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
)

func TestColorExtensionsDoNotLeakBetweenViewers(t *testing.T) {
	for _, count := range []int{1, 3} {
		shared := make([]*rtp.Packet, count)
		for i := range shared {
			shared[i] = &rtp.Packet{Header: rtp.Header{Version: 2, Marker: i == count-1}}
		}
		pq, sdr := [4]byte{9, 16, 9, 16}, [4]byte{1, 1, 1, 16}
		first, err := videoPacketsWithExtensions(shared, 3, 4, pq)
		if err != nil {
			t.Fatal(err)
		}
		second, err := videoPacketsWithExtensions(shared, 5, 6, sdr)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first[count-1].GetExtension(4), pq[:]) || !bytes.Equal(second[count-1].GetExtension(6), sdr[:]) {
			t.Fatal("frame colorimetry lost")
		}
		if !bytes.Equal(first[0].GetExtension(3), []byte{0, 0, 0}) {
			t.Fatal("playout delay lost")
		}
		for i := range shared {
			if shared[i].Extension || second[i].GetExtension(4) != nil || first[i].GetExtension(6) != nil {
				t.Fatal("viewer headers mutated")
			}
			if i != count-1 && (first[i].GetExtension(4) != nil || second[i].GetExtension(6) != nil) {
				t.Fatal("color sent before final packet")
			}
		}
		untagged, err := videoPacketsWithExtensions(shared, 0, 4, [4]byte{})
		if err != nil || untagged[count-1].Extension {
			t.Fatal("fabricated metadata for untagged frame")
		}
		unnegotiated, err := videoPacketsWithExtensions(shared, 0, 0, pq)
		if err != nil || unnegotiated[count-1].Extension {
			t.Fatal("unnegotiated extension emitted")
		}
	}
}

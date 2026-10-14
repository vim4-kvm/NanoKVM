//go:build linux && vim4

package webrtc

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestUDPBatcherPreservesDatagrams(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	_ = receiver.SetReadBuffer(4 << 20)
	batcher, err := newUDPBatcher()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := batcher.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, ok := conn.(*batchConn); !ok {
		t.Fatalf("unicast socket was not batched: %T", conn)
	}

	// A frame-sized burst with an odd first packet, full runs and short tails.
	var sent [][]byte
	for i, size := range append([]int{1408}, append(repeat(1400, 130), 700, 1400, 1400, 33)...) {
		sent = append(sent, bytes.Repeat([]byte{byte(i)}, size))
	}
	write := func(p []byte) {
		if n, err := conn.WriteTo(p, receiver.LocalAddr()); err != nil || n != len(p) {
			t.Fatalf("write %d: %d %v", len(p), n, err)
		}
	}
	write([]byte("before"))
	batcher.begin()
	for _, p := range sent {
		write(p)
	}
	if err := batcher.flush(); err != nil {
		t.Fatal(err)
	}
	write([]byte("after"))

	want := append(append([][]byte{[]byte("before")}, sent...), []byte("after"))
	buffer := make([]byte, 2048)
	_ = receiver.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i, expected := range want {
		n, err := receiver.Read(buffer)
		if err != nil {
			t.Fatalf("datagram %d: %v", i, err)
		}
		if !bytes.Equal(buffer[:n], expected) {
			t.Fatalf("datagram %d: got %d bytes, want %d", i, n, len(expected))
		}
	}
}

func TestUDPBatcherLeavesMulticastSocket(t *testing.T) {
	batcher, err := newUDPBatcher()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := batcher.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 0})
	if err != nil {
		t.Skip("multicast listen unavailable:", err)
	}
	defer conn.Close()
	if _, ok := conn.(*net.UDPConn); !ok {
		t.Fatalf("multicast socket must stay concrete for mDNS: %T", conn)
	}
}

func repeat(value, count int) []int {
	values := make([]int, count)
	for i := range values {
		values[i] = value
	}
	return values
}

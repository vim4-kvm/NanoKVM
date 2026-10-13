//go:build vim4

package network

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestMagicPacketDelivery(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := sendMagicPacket("02:11:22:33:44:55", listener.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatal(err)
	}
	listener.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 256)
	n, _, err := listener.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 102 || !bytes.Equal(buf[:6], []byte{255, 255, 255, 255, 255, 255}) {
		t.Fatalf("invalid header/size: %x", buf[:n])
	}
	for i := 6; i < n; i += 6 {
		if !bytes.Equal(buf[i:i+6], []byte{2, 17, 34, 51, 68, 85}) {
			t.Fatal("wrong target MAC")
		}
	}
}

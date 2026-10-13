//go:build vim4

package network

import (
	"bytes"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"time"
)

func sendWakePacket(mac string) ([]byte, error) {
	return nil, sendMagicPacket(mac, &net.UDPAddr{IP: net.IPv4bcast, Port: 9})
}

func sendMagicPacket(mac string, target *net.UDPAddr) error {
	address, err := net.ParseMAC(mac)
	if err != nil || len(address) != 6 {
		return fmt.Errorf("invalid Ethernet address")
	}
	packet := append(bytes.Repeat([]byte{0xff}, 6), bytes.Repeat(address, 16)...)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		return err
	}
	defer conn.Close()
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var optionErr error
	if err = raw.Control(func(fd uintptr) { optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1) }); err != nil {
		return err
	}
	if optionErr != nil {
		return optionErr
	}
	if err = conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	_, err = conn.WriteToUDP(packet, target)
	return err
}

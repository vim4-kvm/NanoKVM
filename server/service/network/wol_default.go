//go:build !vim4

package network

import "os/exec"

func sendWakePacket(mac string) ([]byte, error) {
	return exec.Command("ether-wake", "-b", mac).CombinedOutput()
}

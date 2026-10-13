//go:build !vim4

package hid

func resetPlatformUSB() (bool, error) { return false, nil }

const (
	HID0 = "/dev/hidg0"
	HID1 = "/dev/hidg1"
	HID2 = "/dev/hidg2"
)

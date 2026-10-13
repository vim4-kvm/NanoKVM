//go:build vim4

package hid

// The board service owns gadget creation and USB-C PD. Recovery retries its
// existing endpoints without running the appliance's USB PHY reset script.
func resetPlatformUSB() (bool, error) {
	h := GetHid()
	h.Lock()
	defer h.Unlock()
	h.CloseNoLock()
	return true, h.OpenNoLockWithRetry(hidReopenTimeout, hidReopenRetryDelay)
}

const (
	HID0 = "/dev/hidg0"
	HID1 = "/dev/hidg2"
	HID2 = "/dev/hidg1"
)

// Reenumerate closes all gadget descriptors before the board helper relinks
// functions. Holding both locks prevents writes and LED reopen during rebinding.
func (h *Hid) Reenumerate(relink func() error) error {
	h.Lock()
	defer h.Unlock()
	h.CloseNoLock()
	return relink()
}

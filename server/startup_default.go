//go:build !vim4

package main

import (
	"NanoKVM-Server/service/vm"
	"NanoKVM-Server/utils"
	"time"
)

func initializeHDMI() {
	vm.DisableHdmiCapture()
	time.Sleep(10 * time.Millisecond)
	if !utils.IsHdmiDisabled() {
		vm.EnableHdmiCapture()
	}
	vm.SetHdmiViewerCount(0)
}

//go:build linux

package stream

import (
	"NanoKVM-Server/internal/platform"
	"syscall"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

func SetCPUAffinity(cpuID int) error {
	// Go may reuse the thread after UnlockOSThread. On VIM4 leave scheduling
	// to Linux instead of retaining a single-CPU mask on a runtime thread.
	if platform.VIM4 {
		return nil
	}
	tid := syscall.Gettid()
	var mask uint64
	mask |= 1 << cpuID

	_, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(tid), unsafe.Sizeof(mask), uintptr(unsafe.Pointer(&mask)))
	if err != 0 {
		log.Errorf("Error setting CPU affinity: %v", err)
		return err
	}

	return nil
}

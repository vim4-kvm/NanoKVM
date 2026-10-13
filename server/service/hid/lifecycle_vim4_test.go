//go:build vim4

package hid

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReenumerateDrainsLEDPollAndClosesEveryEndpoint(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	h := &Hid{g0Reader: reader}
	if err := h.ensureKeyboardLedReaderNotifierNoLock(); err != nil {
		t.Fatal(err)
	}
	defer h.ledReaderNotifyReader.Close()
	defer h.ledReaderNotifyWriter.Close()
	files := []*os.File{reader}
	for _, device := range h.devices() {
		file, err := os.Create(filepath.Join(t.TempDir(), "hid"))
		if err != nil {
			t.Fatal(err)
		}
		device.set(file)
		files = append(files, file)
	}
	started := make(chan struct{})
	polled := make(chan error, 1)
	go func() {
		h.ledPollMutex.Lock()
		close(started)
		_, err := waitForKeyboardLedReportOrChange(reader, h.ledReaderNotifyReadFD)
		polled <- err
		h.ledPollMutex.Unlock()
	}()
	<-started
	relinkError := errors.New("relink failed")
	err = h.Reenumerate(func() error {
		select {
		case err := <-polled:
			if err != nil {
				t.Error(err)
			}
		default:
			t.Error("helper ran before LED poll returned")
		}
		for _, file := range files {
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Errorf("descriptor still open: %v", err)
			}
		}
		return relinkError
	})
	if !errors.Is(err, relinkError) {
		t.Fatal(err)
	}
	if h.g0 != nil || h.g1 != nil || h.g2 != nil || h.g0Reader != nil {
		t.Fatal("stale endpoint retained after helper failure")
	}
}

func TestLastViewerClosesEndpointsOnlyAfterOtherViewerLeaves(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "hid"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	h := &Hid{g1: file, viewers: 2}
	h.Release()
	if _, err := file.Stat(); err != nil {
		t.Fatal("first viewer closed another viewer's endpoint")
	}
	h.Release()
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("last viewer left endpoint open")
	}
	h.Release()
}

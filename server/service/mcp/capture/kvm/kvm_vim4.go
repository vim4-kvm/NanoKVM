//go:build linux && vim4

package kvmcapture

import (
	"NanoKVM-Server/common"
	mcpservice "NanoKVM-Server/service/mcp"
	mcpcapture "NanoKVM-Server/service/mcp/capture"
	"NanoKVM-Server/service/vm"
	"context"
	"time"
)

type snapshotter struct{ slot chan struct{} }

func New() mcpservice.Snapshotter { return &snapshotter{slot: make(chan struct{}, 1)} }

// Keep the official validation, JPEG size limits and lease lifecycle while
// adapting its reader to the context-aware, non-destructive VIM4 snapshot.
func (s *snapshotter) Capture(ctx context.Context, req mcpservice.SnapshotRequest) (mcpservice.Snapshot, error) {
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	case <-ctx.Done():
		return mcpservice.Snapshot{}, ctx.Err()
	}
	timeout := 3000
	if req.TimeoutMS != nil {
		timeout = min(max(*req.TimeoutMS, 0), 30000)
	}
	if timeout == 0 {
		timeout = 3000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	reader := snapshotReader{ctx: ctx}
	capture := mcpcapture.NewWithCaptureLease(reader, func() (uint16, uint16) { return 0, 0 }, vm.AcquireHdmiCaptureLeaseForRead)
	return capture.Capture(ctx, req)
}

type snapshotReader struct{ ctx context.Context }

func (r snapshotReader) ReadMjpeg(w, h, q uint16) ([]byte, int) {
	d, err := common.GetKvmVision().SnapshotImage(r.ctx, w, h, q)
	if err != nil {
		return nil, -1
	}
	return d, 0
}

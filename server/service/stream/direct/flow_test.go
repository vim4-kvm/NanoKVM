package direct

import "testing"

func TestSkippedCaptureRequiresIDRWithoutResettingFlowWindow(t *testing.T) {
	q := newFrameQueue(8, 1024)
	q.enableFlowControl(1)
	if !q.offer(newOutboundFrame(true, 1, []byte{1})) {
		t.Fatal("IDR rejected")
	}
	q.popForWrite()
	q.discardPredictionChain()
	if q.canAdvanceStream() {
		t.Fatal("unacknowledged frame lost from flow window")
	}
	q.acknowledge(1)
	if q.offer(newOutboundFrame(false, 2, []byte{2})) {
		t.Fatal("prediction frame accepted after skipped capture")
	}
	if !q.offer(newOutboundFrame(true, 3, []byte{3})) {
		t.Fatal("IDR failed to recover")
	}
}

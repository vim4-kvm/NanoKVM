//go:build !(linux && vim4)

package webrtc

import "github.com/pion/transport/v3"

type udpBatcher struct{ transport.Net }

func newUDPBatcher() (*udpBatcher, error) { return nil, nil }
func (*udpBatcher) begin()                {}
func (*udpBatcher) flush() error          { return nil }

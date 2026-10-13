//go:build vim4

package router

import (
	"NanoKVM-Server/proto"
	"testing"
)

func TestTailscaleStatusMapsToOriginalStates(t *testing.T) {
	for _, c := range []struct {
		status tailscaleStatusVIM4
		want   proto.GetTailscaleStatusRsp
	}{
		{tailscaleStatusVIM4{}, proto.GetTailscaleStatusRsp{State: proto.TailscaleNotInstall}},
		{tailscaleStatusVIM4{Installed: true}, proto.GetTailscaleStatusRsp{State: proto.TailscaleNotRunning}},
		{tailscaleStatusVIM4{Installed: true, Running: true, BackendState: "NeedsLogin"},
			proto.GetTailscaleStatusRsp{State: proto.TailscaleNotLogin}},
		{tailscaleStatusVIM4{Installed: true, Running: true, BackendState: "Running", HostName: "vim4",
			IPs: []string{"fd7a:115c:a1e0::1", "100.64.0.7"}, Tailnet: "example.ts.net"},
			proto.GetTailscaleStatusRsp{State: proto.TailscaleRunning, Name: "vim4", IP: "100.64.0.7", Account: "example.ts.net"}},
	} {
		if got, ok := tailscaleStatusRsp(c.status); !ok || got != c.want {
			t.Errorf("%+v: got %+v", c.status, got)
		}
	}
	if _, ok := tailscaleStatusRsp(tailscaleStatusVIM4{Installed: true, Running: true, BackendState: "Bogus"}); ok {
		t.Error("accepted an unknown backend state")
	}
}

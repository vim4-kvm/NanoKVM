//go:build vim4

package picoclaw

import "testing"

func TestGatewayProcessDoesNotAdoptUnrelatedInstallation(t *testing.T) {
	for _, command := range []string{"/usr/bin/picoclaw\x00gateway\x00", "/tmp/picoclaw\x00gateway\x00", "picoclaw\x00gateway\x00"} {
		if isPicoclawGatewayCmdline([]byte(command)) {
			t.Fatalf("unrelated process accepted: %q", command)
		}
	}
}

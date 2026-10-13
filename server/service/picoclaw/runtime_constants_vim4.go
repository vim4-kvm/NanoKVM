//go:build vim4

package picoclaw

import "time"

const (
	picoclawPlatformHome    = "/var/lib/vim4-kvm/picoclaw/home"
	picoclawBinaryPath      = "/var/lib/vim4-kvm/picoclaw/bin/picoclaw"
	picoclawCacheDir        = "/var/lib/vim4-kvm/picoclaw/cache"
	picoclawPIDFileName     = ".picoclaw.pid"
	picoclawDownloadURL     = "https://github.com/sipeed/picoclaw/releases/download/v0.2.8/picoclaw_Linux_arm64.tar.gz"
	picoclawChecksumURL     = ""
	picoclawPinnedSHA512    = "d0f49493dda0ee725051e6bea4af8e795096806cdf864eaa59ab7b12dc398dcf1d1dabb1257f0a35467adc633cb0ab16a0ff76f21e8a6f8f0437be7c2e3e11c9"
	etcInitPicoclawScript   = "/usr/libexec/vim4-picoclaw"
	kvmappPicoclawScript    = "/opt/vim4-nanokvm/deploy/vim4/picoclaw"
	picoclawStartTimeout    = 15 * time.Second
	picoclawStopTimeout     = 15 * time.Second
	picoclawOnboardTimeout  = 60 * time.Second
	picoclawInstallTimeout  = 12 * time.Minute
	picoclawDownloadTimeout = 10 * time.Minute
	picoclawStartWaitPeriod = 1500 * time.Millisecond
	picoclawStopWaitPeriod  = 500 * time.Millisecond
	picoclawReadyPollPeriod = 500 * time.Millisecond
)

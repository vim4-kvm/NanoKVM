//go:build vim4

package utils

// systemd creates this directory for the unprivileged application user.
// Updates and image transfers share the same lock and restart clears it.
const TransferSentinelPath = "/run/vim4-nanokvm/transfer-in-progress"

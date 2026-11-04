# NanoKVM on VIM4

This page covers Ubuntu integration constraints; upstream product documentation stays upstream.
Read [build](#build), [runtime](#runtime-boundaries), [media](#shared-media), or [validation](#validation-limits).
Board services own HDMI, gadget, PD and clocks; applications share hardware and run one at a time.

## Build

Use the public workspace [build tools](https://github.com/vim4-kvm/vim4-kvm/blob/main/docs/tools.md)
and [package definitions](https://github.com/vim4-kvm/vim4-kvm/blob/main/packaging/ubuntu/package.py).
Install matching board, capture, encoder and GPU components before the application; a web update cannot replace platform dependencies.
Backend selection and plugin precedence belong to
[vim4-kvm-run](https://github.com/vim4-kvm/vim4-kvm/blob/main/board/vim4-kvm-run).

For local development, run the VIM4 Go suite in server and the locked frontend build in web.
Dependencies and build options are authoritative in [server/go.mod](server/go.mod),
[web/package.json](web/package.json) and [service](deploy/vim4/nanokvm.service).
The default target retains libkvm; the workspace ABI-check stub must never enter a device package.
Updater naming is retained for installed-client compatibility; see [package.py](deploy/vim4/package.py).
No public update source is configured.

## Runtime boundaries

[normalizePlatformConfig](server/config/platform_vim4.go) enforces the loopback gateway contract in memory,
including old configurations, without rewriting user YAML. Manager login owns public access;
MCP still needs its own API key. Ubuntu passwords are unchanged.
Tailscale installation and login belong to the manager, independently of application selection.

Streaming settings and codec limits come from [video settings](server/common/video_settings_vim4.go)
and [stream implementation](server/service/stream).
Higher bitrate targets require bandwidth and do not guarantee output rate; deploy the matching encoder plugin.
Offloaded UDP bursts are deliberately bounded: larger bursts lost packets during wired motion tests despite spare average bandwidth.
Keep this constraint when tuning syscall overhead; it is not congestion control.
See [batch_vim4.go](server/service/stream/webrtc/batch_vim4.go), including multicast handling.

Vendor input changes rebuild capture without replacing media connections. Unchanged pixels are not a stall;
the watchdog watches missing buffers. Matched capture pools and encoder workspace reservation leave contiguous memory for mode changes.
HDMI resynchronization still interrupts new frames. Base-rate fallback does not qualify VRR.
Mainline EDID changes release capture first; use the SDR-compatible preset selected by
[EDID implementation](deploy/vim4/vim4-edid), since the old vendor table can select an unsupported transport on macOS.

HDR capability comes from the shared source's probe rather than a kernel release-name check.
Wave5 Main10 snapshots use the source's raw-frame path instead of opening another VDIN or stopping the encoder.
The vendor backend retains independent SDR capture. H.264, MJPEG and snapshots stay SDR;
native HDR requires negotiated decoder support, otherwise WebRTC falls back to H.264.
Direct's native video path may present HDR; ordinary Canvas remains SDR. Safari remote HDR is unqualified.
Resolution controls must not restart the HDMI display.
Probe and pipeline details belong to [capture](server/common/screen_vim4.go) and
[snapshot](server/common/snapshot_vim4.go), with acceptance evidence in
[HDR records](https://github.com/vim4-kvm/vim4-kvm/blob/main/docs/hdr.md#相关记录).

Unattached ATX/display hardware stays unconfigured. Application passthrough and disabling the board-owned HDMI source are not implemented.
Microphone capture needs a secure context and exchanges USB networking for UAC2.
Keep board services running and never delete an active encoder lock.

## Shared media

Startup does not attach the shared disk; host-ejected media stays ejected across switches and restarts.
Attach explicitly when needed. The legacy restore command remains a compatibility no-op.
The catalog groups foreign images and shared data disks: foreign images are read-only,
data disks can be writable here; deletion and rename belong to their source.
The legacy disk switch targets only its own disk.

An occupied LUN cannot be replaced. Only a successful mount owned by the running NanoKVM process may be ejected here.
After restart, or for media attached elsewhere, safely eject on the target computer and use the manager.
The helper rechecks the expected path under shared locks and refuses data disks being edited.
See [vim4-media](deploy/vim4/vim4-media); install the matching
[shared board module](https://github.com/vim4-kvm/vim4-kvm/blob/main/board/vim4media.py) first.

## Validation limits

Check both VIM4 and default builds, helper suites and the locked frontend build through the workspace.
Browser [input](web/tests/input.mjs) and [Wi-Fi](web/tests/wifi.mjs) tests use mocks and do not inject HID.
Wi-Fi scanning failure or hidden SSIDs must retain manual entry.

Earlier hardware results do not qualify the current checkout. Video/audio, USB switching,
reboot and fresh-image installation need an exclusive board regression. Actual AI execution needs user credentials.
HEVC requires browser decoder support; Direct and microphone require a secure context.
Signaling contracts: [stream notes](server/service/stream/VIM4.md).
Historical evidence: [integration record](https://github.com/vim4-kvm/vim4-kvm/blob/main/docs/bringup/2026-10-13-nanokvm-deploy-jetkvm-switch.md).

# NanoKVM on VIM4 / Ubuntu 24.04

The `vim4` branch keeps NanoKVM's desktop, accounts/roles, MCP, PicoClaw,
control regions and mobile gestures. Build-tagged adapters replace hardware and
system operations on Ubuntu; the default build retains libkvm.

The board's `vim4-kvm-run` launcher selects vendor capture/encoding on 5.15 and
`vim4v4l2src` / `vim4wave5enc` on the matched 7.x kernel. HDMI audio selects the
corresponding ALSA card. Both kernels use the same NanoKVM configuration and
image catalog; install the board package providing `vim4-runtime-api-1` with
the matching mainline plugins before updating this application.

Native EDID changes on 7.x release capture before programming the receiver.
Use `VIM4-1080P60-SDR8` for 1080p: the original vendor preset can make macOS
select an unsupported 10-bit transport. Original and uploaded tables are retained.

## Build

Use the adjacent vim4-kvm workspace and its pinned Ubuntu ARM64 builder:

```sh
python3 tools/workspace.py build all
python3 tools/ubuntu-build.py package --version VERSION
python3 tools/ubuntu-build.py verify --version VERSION
```

Server: `go test -tags vim4 ./...`, then `go build -tags vim4 .` on Linux ARM64
with GStreamer, ALSA/Opus and vendor encoding dependencies. Web:
`pnpm install --frozen-lockfile && VITE_PLATFORM=vim4 pnpm build`.
The default Go target requires the original device library; the workspace
builder checks its ABI with an aborting stub, never included in device packages.

## Runtime boundaries

Custom H.264/H.265 bitrate accepts 1000–60000 kbps; the default remains 12000.
Deploy the matching vim4-amlvenc plugin with the applications: older builds
limit bitrate to 12000 kbps. Higher targets require more network bandwidth;
actual output depends on the scene and rate-control mode.

WebRTC sends at most eight UDP segments per offloaded batch. Larger bursts
produced receiver packet loss during wired 4K60/60 Mbps motion tests despite
available average bandwidth; keep this bound when tuning syscall overhead.
Unsupported offload and IPv6 retain the per-packet path. This is a burst bound,
not congestion control or a guarantee for slower networks.

The Screen → Quality menu offers H.264/H.265 presets from 1 to 60 Mbps and a
slider in 0.1 Mbps steps. Dragging previews the value; releasing saves it through
the video settings API. Refreshing reads the saved device bitrate, including
custom values between presets. MJPEG retains separate JPEG quality choices.

| Area | VIM4 implementation |
|---|---|
| H.264/H.265 WebRTC and Direct | Shared DMA-buf capture and hardware encoder, PTS, keyframe requests, bounded viewer queues |
| MJPEG | DMA hardware JPEG via vim4jpegenc, up to the saved FPS/source limit; software fallback capped at 10 fps when the plugin is unavailable |
| Audio | One shared HDMI ALSA reader, independent Opus WebRTC for every video mode |
| Microphone | Browser audio to USB UAC2; requires a secure context and exchanges USB networking for UAC2 |
| Capture controls | Resolution, FPS, bitrate, GOP, CBR/VBR, EDID and rotation in the original menu/settings layout |
| HID | Native input arbitration, VIM4 reports, close-before-rebind and LED polling |
| Storage | Validated image uploads/downloads and virtual disk/CD-ROM helper |
| Ubuntu management | NetworkManager, DNS, chrony, SSH, mDNS, hostname, time and reboot through restricted helpers |
| Tailscale | Managed for the whole board by vim4-manager; no application menu or API routes |
| Accounts | vim4-manager gateway login; the loopback backend disables web authentication and hides account settings; no Ubuntu password changes |
| Automation | Native MCP/PicoClaw, ARM64 runtime in an application-owned directory |
| Updates | Validated `vim4-nanokvm` ARM64 deb bundles; no appliance firmware installation |

Only HDMI and USB-C are connected. ATX, external displays and other unattached
hardware report unconfigured. HDMI passthrough and disabling the board-owned
HDMI source are not implemented. Keep board HDMI, PD, gadget and clock services.
Applications listen on 127.0.0.1:8081 behind vim4-manager's authenticated 8080
gateway and share the same hardware. The VIM4 server enforces host, port.http
and proto in memory after loading server.yaml, including older configurations.
It logs any override and disables TLS listeners and HTTP-to-HTTPS redirects;
package upgrades preserve the existing file without a YAML migration dependency.
MCP still requires its own API key even when web authentication is disabled.
The manager selects
one; packages preserve that selection. Never delete the active encoder lock.

Starting NanoKVM does not attach the shared USB data disk. Host-ejected media
stays ejected across application switches and restarts; existing image files
are retained. Connect it explicitly from the storage/USB controls when needed.
The legacy `vim4-media disk restore` command is a no-op for older service units.

The image list also groups other KVM applications' images and every shared data
disk by source. Foreign images mount read-only (ISO as CD-ROM); data disks mount
writable from this UI. Delete and rename remain the source application's job.
The virtual-disk switch still targets only `disk.img`. Both mount paths use the
board's `vim4media` module and both global locks, and refuse disks being edited
in the manager. Install the matching board module at
`/usr/lib/vim4-kvm/python/vim4media.py` before using these helpers.
An occupied LUN cannot be replaced. Eject is allowed only for a successful mount
owned by the current NanoKVM process; the helper rechecks its expected path under
the shared locks. Media attached elsewhere or retained across a restart must be
ejected through the manager. The legacy disk switch cannot eject another disk.

## Installation and updates

Use workspace `tools/board.py deploy --exclusive --packages ...`.
Installed paths and deb identity remain `/opt/vim4-nanokvm`, `nanokvm.service`
and `vim4-nanokvm` for upgrades. Packages include `/usr/libexec` helpers, matching
sudoers, web/PicoClaw assets and service definitions.

`deploy/vim4/package.py` wraps the application deb for Manual Update/custom
servers. The `nanokvm_pro_VERSION.tar.gz` and `nanokvm_pro_latest.json` names
remain for installed updater compatibility. No public update source is set.
Platform dependency changes require complete component deployment first.

Settings → Network → Wi-Fi scans nearby networks when opened and supports
refreshing the list. Names are grouped by SSID and sorted by strongest signal;
select a network and enter its password, or leave it empty for an open network.
Manual SSID entry remains available when scanning fails or a network is hidden.

## Validation limits

Software checks cover both backend builds, VIM4/default Go suites, helper tests,
frontend builds and browser input regressions. Input tests mock the WebSocket
and do not inject input into the attached Mac:

```sh
VITE_PLATFORM=vim4 pnpm dev --host 127.0.0.1 --port 5179
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs node tests/input.mjs
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs node tests/wifi.mjs
```

Earlier hardware results do not qualify this branch. Video/audio playback,
USB switching, reboot and fresh-image installation require an exclusive board
regression. Actual AI execution requires user credentials. Install and log in
to Tailscale through vim4-manager, whose helper manages the system tailscaled;
application upgrades preserve the installed package and login state.
H.265 requires browser decoder support; Direct/microphone require a secure
context. See `server/service/stream/VIM4.md` for signaling/settings contracts.

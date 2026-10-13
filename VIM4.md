# NanoKVM on VIM4 / Ubuntu 24.04

The `vim4` branch keeps NanoKVM's desktop, accounts/roles, MCP, PicoClaw,
control regions and mobile gestures. Build-tagged adapters replace hardware and
system operations on Ubuntu; the default build retains libkvm.

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

| Area | VIM4 implementation |
|---|---|
| H.264/H.265 WebRTC and Direct | Shared DMA-buf capture and hardware encoder, PTS, keyframe requests, bounded viewer queues |
| MJPEG | Software JPEG, at most 10 fps; not equivalent to hardware 4K60 |
| Audio | One shared HDMI ALSA reader, independent Opus WebRTC for every video mode |
| Microphone | Browser audio to USB UAC2; requires a secure context and exchanges USB networking for UAC2 |
| Capture controls | Resolution, FPS, bitrate, GOP, CBR/VBR, EDID and rotation in the original menu/settings layout |
| HID | Native input arbitration, VIM4 reports, close-before-rebind and LED polling |
| Storage | Validated image uploads/downloads and virtual disk/CD-ROM helper |
| Ubuntu management | NetworkManager, DNS, chrony, SSH, mDNS, hostname, time and reboot through restricted helpers |
| Accounts | Native roles/sessions; existing credentials and disabled-login preference retained; no Ubuntu password changes |
| Automation | Native MCP/PicoClaw, ARM64 runtime in an application-owned directory, optional assistant venv |
| Updates | Validated `vim4-nanokvm` ARM64 deb bundles; no appliance firmware installation |

Only HDMI and USB-C are connected. ATX, external displays and other unattached
hardware report unconfigured. HDMI passthrough and disabling the board-owned
HDMI source are not implemented. Keep board HDMI, PD, gadget and clock services.
NanoKVM and JetKVM both use port 8080 and the same hardware. The manager selects
one; packages preserve that selection. Never delete the active encoder lock.

## Installation and updates

Use workspace `tools/board.py deploy --exclusive --packages ...`.
Installed paths and deb identity remain `/opt/vim4-nanokvm`, `nanokvm.service`
and `vim4-nanokvm` for upgrades. Packages include `/usr/libexec` helpers, matching
sudoers, web/PicoClaw assets and service definitions.

`deploy/vim4/package.py` wraps the application deb for Manual Update/custom
servers. The `nanokvm_pro_VERSION.tar.gz` and `nanokvm_pro_latest.json` names
remain for installed updater compatibility. No public update source is set.
Platform dependency changes require complete component deployment first.

## Validation limits

Software checks cover both backend builds, VIM4/default Go suites, helper tests,
frontend builds and browser input regressions. Input tests mock the WebSocket
and do not inject input into the attached Mac:

```sh
VITE_PLATFORM=vim4 pnpm dev --host 127.0.0.1 --port 5179
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs node tests/input.mjs
```

Earlier hardware results do not qualify this branch. Video/audio playback,
USB switching, reboot and fresh-image installation require an exclusive board
regression. Tailscale login and actual AI execution require user credentials.
H.265 requires browser decoder support; Direct/microphone require a secure
context. See `server/service/stream/VIM4.md` for signaling/settings contracts.

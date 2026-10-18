# PicoClaw and MCP integration

The VIM4 application uses the existing video pipeline's `Snapshot(ctx)` method
for both MCP and PicoClaw screenshots. Snapshot processing can resize and
recompress JPEGs but must not call `ReadMjpeg` or change video transport.
PicoClaw is the supported AI assistant; the legacy Python assistant is removed on upgrade.

## Runtime package

The installer downloads the ARM64 v0.2.8 release archive from
https://github.com/sipeed/picoclaw/releases/download/v0.2.8/picoclaw_Linux_arm64.tar.gz.
Its SHA-256 is
`9ce9fbf76da9c967baffb18c4b502dec2055b3ad050fc2381b6260429e1acf70`.
The application pins the corresponding SHA-512 in
`server/service/picoclaw/runtime_constants_vim4.go` and verifies the complete
archive before extracting or installing the binary. Version updates require
reviewing and changing both the URL and pinned digest.

Package/install hooks:

- Install `deploy/vim4/picoclaw` as root-owned mode 0755
  `/usr/libexec/vim4-picoclaw`. It runs as khadas, without sudo.
- Copy the repository's `kvmapp/picoclaw/` directory, including both agent profiles and
  `skills/kvm-control`, to `/opt/vim4-nanokvm/picoclaw` with readable files and
  traversable directories. Keep scripts executable.
- Create `/var/lib/vim4-kvm/picoclaw` with owner/group khadas and mode 0700.
  The application creates `bin/`, `cache/` and `home/` underneath it.
- Ensure khadas can write `/etc/kvm`, including `ai-control.mode`, `mcp.json`,
  `.picoclaw_internal_token`, and the runtime-intent file. Token/config files
  are created with restricted permissions by the service.
- Runtime package dependencies: `python3`, `curl`, `ca-certificates` and the
  existing shell/core utilities. The downloaded PicoClaw ELF is statically
  linked ARM64. No RISC-V emulator, SysV init, or root runtime is needed.
- Keep systemd's default `KillMode=control-group` for `nanokvm.service`. The
  helper's gateway child remains in that cgroup and is terminated on application
  shutdown. Runtime intent reconciliation restores it when requested.
- Keep any configured `PICOCLAW_HOME` identical for server and helper. The VIM4
  default is `/var/lib/vim4-kvm/picoclaw/home`. The helper supplies
  `NANOKVM_BRIDGE_BASE_URL=http://127.0.0.1:8081`; set it explicitly if the
  application port is changed. The MCP URL itself uses configured HTTP port.

There is no additional systemd service or sudoers entry. Configuration, model
credentials and conversation data are writable only by the application user.
Gateway logs are in `/var/lib/vim4-kvm/picoclaw/runtime.log`.

## Application contracts

The original router creates one manager/service pair during route setup:

```go
control := controlmode.GetManager()
picoclawService := picoclaw.NewService(control)
controlRouter(r, control, picoclawService)
mcpRouter(r, control, picoclawService)
picoclawRouter(r, picoclawService)
```

Allow `PicoclawLoopbackHTTPAllowedPaths()` through the HTTPS redirect middleware
only for real loopback clients. Each internal endpoint still requires the
dedicated token through `middleware.CheckLoopbackInternalToken()`. External MCP
always requires its bearer key, including when web password authentication is
disabled. Management endpoints require the admin role.

Required configuration APIs are `config.GetPicoclawInternalToken()`,
`config.EnsurePicoclawInternalToken()` and `config.PicoclawInternalTokenHeader`.
Middleware also supplies `CheckToken`, `RequireRole(authn.RoleAdmin)` and
`WatchWebSocket` so revoked sessions close active gateway connections.

Go dependencies are `github.com/modelcontextprotocol/go-sdk v1.6.1` and
`github.com/google/jsonschema-go v0.4.3`; resolve their transitive dependencies
without downgrading the existing VIM4 requirements.

HID must expose error-returning keyboard, relative mouse and absolute mouse
report methods and `(*Hid).ReleaseAllHIDStateBestEffort() error`. Services use
`hid.GetHid().ReleaseAllHIDStateBestEffort` as their cleanup callback. Cleanup
attempts all endpoints and VIM4 releases preserve the last absolute pointer
position.

Each WebSocket client owns an `inputcontrol.ManualSession`. Reserve a report
before enqueueing with `ReserveWithCooldown`, execute its write through
`manual.Execute`, then call `reservation.Complete(success)`. Dropped queue items
must complete with false. Reset the held-state kind after failed writes are
released, and close the manual session after draining its queues. The admission
predicate is `mode != controlmode.ModePicoclaw ||
!picoclaw.GetSessionLock().BlocksManualInput()`. Preserve HID Acquire/Release and
last-viewer cleanup. Apply this arbitration to other manual writers (paste,
shortcuts and jiggler) as appropriate; do not let them bypass active AI writes.

## Frontend integration

The original frontend already contains `react-markdown`, `remark-gfm`, icons,
gateway/storage helpers, API modules and Jotai state. Preserve these integrations:

- Keep `<MCP />` from `menu/settings/mcp` as the admin-only `mcp` settings item.
- Keep `<Picoclaw />` from `menu/picoclaw` as the admin-only `picoclaw` menu item.
  Include `picoclaw` in the menu configuration/defaults if it filters item IDs.
- Desktop imports `Sidebar as PicoclawSidebar`, `ActionOverlay`, and
  `picoclawChatOpenAtom`. Use the existing desktop split-pane layout for the
  desktop sidebar (initial 420px, minimum 340px, maximum 45%) and an overlay on
  small screens; render `ActionOverlay` over the video region. Keep the sidebar
  mounted/open according to that atom and preserve all VIM4 video/input elements.
- Merge the source `picoclaw` and `settings.mcp` translation trees, plus
  shared referenced labels such as `menu.collapse`. The settings tab title is
  `settings.mcp.title`. Feature files contain no
  newly invented translation keys.
- Preserve the keyboard focus-lock API in `jotai/keyboard`: the feature calls
  `keyboardLockAtom` with `{source, locked}` so chat typing never reaches USB.

## Validation boundaries

Container-only onboarding can validate the binary architecture, config schema,
agent profile and skill installation without a model key. Gateway/model inference,
actual USB actions and screenshots still require separately authorized runtime
or hardware tests. An onboarding pass is not an inference or hardware pass.

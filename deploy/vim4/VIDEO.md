# Encoder budget and packet sizing

The workspace Ubuntu `vim4-nanokvm` package depends on `vim4-amlvenc`, which
installs the plugin and service drop-in under `/usr/lib`. For manual installs,
build/install the sibling `gst-plugin-venc` repository on its `vim4` branch
before deploying the updated application. Its `install-vim4.sh` installs the
plugin under `/usr/local/lib/vim4-kvm/gstreamer-1.0` and creates a NanoKVM service
drop-in. Do not overwrite the vendor plugin. Legacy `deploy/vim4/package.py`
application-only bundles still require separately installed platform dependencies;
the workspace Ubuntu component packages include the encoder dependency.

The application reads the HDMI receiver's hundredths-of-fps rate at pipeline
creation, rounds it for the integer codec API, and caps it at the requested
drop-only FPS. Unknown HDMI rate falls back to 30. vfm_cap currently advertises
unspecified frame rate, so the app supplies it explicitly to amlvenc. A change
in source rate is checked every 500ms. Three consecutive equal, valid readings
with a different effective budget cancel capture; the normal retry path reopens
it (including its one-second retry delay). Loss of HDMI lock does not trigger
repeated restarts. Frame-rate transitions may briefly interrupt video. This does not
alter capture timestamps or add pacing/buffering. Keep GOP 120 and performance
governor; PLI recovery remains enabled.

At pipeline creation, the application checks the loaded plugin's writable integer
`framerate` range. Unsupported budgets are omitted, preserving the old plugin's
default and logging a warning: video can still work, but target bitrate accuracy
is not guaranteed. This is a compatibility fallback, not a replacement for the
packaged encoder dependency. VIM4 leaves CPU affinity to Linux so unlocked Go
runtime threads do not retain the stream's single-CPU affinity mask.

`VIM4_RTP_PACKET_SIZE` optionally selects a 1000–1400-byte packetizer budget.
Unset/invalid values use 1200. RTP extensions, SRTP and network headers are
additional; this number is not the Ethernet MTU. For the tested direct LAN with
MTU 1500:

```sh
sudo install -m 0644 deploy/vim4/rtp-lan.conf /etc/systemd/system/nanokvm.service.d/rtp-lan.conf
sudo systemctl daemon-reload
sudo systemctl restart nanokvm
```

Remove that specific drop-in and restart the service to restore 1200 for an
unknown route, VPN or smaller path MTU. The generic installer and release bundle
do not enable this device-specific override automatically. No frontend changes
are required.

## Validation

The same scrolling text/color animation was displayed on the Mac HDMI output;
Chrome headless received 3840×2160. Each measurement covers about 20 seconds,
after connection warmup, with bitrate 12000 and GOP 120:

| Metric | Old 30fps budget, 1200 | 60fps budget, 1200 | 60fps budget, 1400 |
| --- | ---: | ---: | ---: |
| Received Mbps | 24.51 | 12.31 | 12.32 / 12.42 |
| Decoded fps | 59.99 | 59.99 | 59.99 / 59.99 |
| Lost packets / dropped decoded frames / freezes | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 |
| RTP packets | 52192 | 26499 | 22822 (first run) |
| Mean assembly time, ms | 1.41 | 0.63 | 0.54 (first run) |
| Decode duration p99, ms, sampled at presentation | 5.6 | 4.6 | 4.6 / 4.6 |
| Decoded inter-frame standard deviation, ms | 6.08 | 3.42 | 3.54 / 3.74 |

Browser presentation intervals were noisier: maximum 49.9ms before, 35.3ms
after the rate fix, and 33.9/47.5ms in the two 1400 runs. Therefore packet-count
and assembly improvements are verified, but a consistent reduction in display
tail latency from 1400 alone is not established. Headless compositor callbacks
are not capture-to-display latency measurements. No universal zero-stutter or
glass-to-glass latency guarantee is claimed.

The 12Mbps setting now targets 12Mbps rather than approximately 24Mbps at 60fps;
high-motion compression quality may differ from the formerly oversized budget.
No fixed-QP, frame-skipping rate control, intra refresh or packet batching is
enabled by default. Operator-only experiments can set `VIM4_ENCODER_I_QP_MIN`
(0–51), `VIM4_ENCODER_INTRA_REFRESH_ROWS` (0–135, also bounded by picture height),
or `VIM4_ENCODER_GOP` (1–1000) in a temporary service drop-in. The loaded plugin
must support each property; invalid or unsupported overrides are ignored with
a warning. These overrides do not change the UI or persisted stream settings.
Use the service log to confirm effective experimental settings. Remove the
drop-in and restart after testing. Image quality, motion and recovery must be
validated before choosing production defaults.

## Opt-in pipeline timing

Set `VIM4_VIDEO_TIMING=1` in a temporary service override and restart NanoKVM to
record per-frame `VIM4 timing read` and `VIM4 timing send` messages. Correlate them
by `pts_ns` within one capture session; PTS resets when capture restarts.
`capture_to_copy_ms` starts at vfmcapsrc's userspace timestamp and includes
encoding, appsink residence and copying. It does not start at HDMI acquisition.
An unavailable age is reported as -1. `copy_to_read_ms` measures the Go handoff;
`send_ms` covers packetizing and sending to all current viewers, not wire arrival.
Filter by viewer count and distinguish keyframes using the read record.

The capture plugin additionally logs kernel delivery-to-dequeue time with
`GST_DEBUG=vfmcapsrc:4`. Its V4L2 timestamp marks delivery, not exposure. Remove
diagnostic overrides and restart after measurement: logging can perturb timings.
Normal service operation performs no extra diagnostic clock queries or logging.

## WebRTC send path

Each RTP packet sent through pion cost one `sendto` (about 12 us of kernel CPU
on this board) plus SRTP. Large 4K P-frames (about 300 KB, 250 packets) took
9 ms to leave the board at about 280 Mbps, often longer than a frame interval,
which appeared as browser frame-interval jitter.

- The board answers as DTLS server and so selects AES-GCM SRTP, which the CPU
  accelerates (4 us versus 9 us per packet for AES-CM/HMAC with Chrome).
- The playout-delay extension is added to the first packet of a frame only.
- Each video connection uses its own UDP sockets. While a frame is written,
  their datagrams are collected and equal-sized runs are sent with
  `UDP_SEGMENT` (up to 64 segments / 64 KB per call). Refused segmentation
  falls back to per-packet sends. Multicast (mDNS) sockets are not wrapped:
  `ipv4.PacketConn` needs the concrete socket, otherwise its read loop spins.

Moving 4K desktop, 30 Mbps CBR, Chrome, 45 s each, zero loss/drops:

| Metric | Before | After |
| --- | ---: | ---: |
| Large-frame send time p50 / p99, ms | 9.0 / 24.7 | 3.6 / 9.2 |
| Last packet behind capture clock p99, ms | 25.2 | 10.3 |
| Browser inter-frame delay SD, ms | 8.1 | 5.9 |

With the new path, the SD is 3.6 ms at 12 Mbps, 2.4 ms at 6 Mbps and 1.8 ms at
3 Mbps. Remaining variation is mostly encode time, which grows with frame
size (about 12 ms small P, 14.5 ms 300 KB P, 18.5 ms IDR at 4K). GOP 600
did not help at 30 Mbps: rate control spent the savings on larger P-frames.

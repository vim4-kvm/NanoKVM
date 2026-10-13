# VIM4 stream API

Build with `-tags vim4` on Linux with CGO, GStreamer app/video development
libraries and libopus. No extra Go modules are required. Untagged builds keep
the upstream libkvm implementation.

The router owns authentication and authorization; handlers below must be wired
through the existing stream middleware. WebSocket handlers also retain origin
checking and the session watcher.

| Route | Handler |
| --- | --- |
| `/api/stream/h264` (existing) | `webrtc.Connect(c *gin.Context)` |
| `/api/stream/h265/webrtc` | `webrtc.ConnectH265(c *gin.Context)` |
| `/api/stream/audio/webrtc` | `webrtc.ConnectAudio(c *gin.Context)` |
| `/api/stream/h265/direct` | `direct.ConnectH265(c *gin.Context)` |

H.264/H.265 WebRTC use the original `video-offer`, `video-answer`,
`video-candidate`, `ice-servers` and `heartbeat` JSON envelopes. Audio uses
`audio-offer`, `audio-answer`, `audio-candidate`, `ice-servers` and `heartbeat`;
the offer is Opus sendrecv for HDMI playback plus microphone input. The envelope
is `{ "event": "...", "data": "<JSON string>" }`. Audio is active in every
video mode and has one shared HDMI reader. Video sessions never read audio.
Only one microphone sender owns the USB playback decoder at a time.

Both Direct codecs use the upstream framing and flow protocol unchanged:
one keyframe byte, eight bytes of little-endian microsecond timestamp, then
Annex-B access-unit bytes. `?flow=N` negotiates the bounded ACK window (1–8).
Client binary message `2` plus the eight-byte timestamp acknowledges frames;
single-byte `3` requests resynchronization. Codec selection is by route.

`stream.NewService()` provides VIM4 settings handlers, each taking
`c *gin.Context`, with the standard NanoKVM response envelope:

| Method/path under `/api/stream` | Handler | JSON body |
| --- | --- | --- |
| GET `/settings` | `GetSettings` | — |
| GET `/snapshot` | `Snapshot` | — (returns JPEG, 503 if unavailable) |
| POST `/resolution` | `SetResolution` | `width`, `height` |
| POST `/rate-control` | `SetRateControlMode` | `mode`: `cbr` or `vbr` |
| POST `/fps` | `SetFps` | `fps`: 0–60 (0 = input rate) |
| POST `/gop` | `SetGop` | `gop`: 1–200 |
| POST `/quality` | `SetQuality` | `quality`: JPEG 1–100 or bitrate 1000–12000 Kbps |
| POST `/mode` | `SetMode` | `mode`: `mjpeg`, `h264-webrtc`, `h264-direct`, `h265-webrtc`, `h265-direct` |

Settings response fields: `width`, `height`, `bitrate`, `gop`, `fps`, `quality`,
`rateControl`, `actualFPS` (measured over three seconds). Supported resolutions
are 1280×720, 1920×1080 and 3840×2160; MJPEG is capped at 10 fps. The configured
FPS is a drop-only ceiling, not a promise to produce that many frames.

Video settings live at `/etc/kvm/video.json`; resolution changes call
`sudo -n /usr/libexec/vim4-video WIDTH HEIGHT`. Capture requires vfmcapsrc,
amlvenc and the HDMI board service. Optional encoder properties are probed at
runtime so older plugins fall back without making the pipeline invalid.
`VIM4_RTP_PACKET_SIZE` sets the H.264/H.265 packetizer budget (1000–1400 bytes,
default 1200). This budget precedes RTP extensions and SRTP, and is not link MTU.
Negotiated playout-delay is zero; PLI/FIR and Direct resync request forced IDR.
Capture PTS supplies RTP and Direct timestamps with continuity across encoder
restarts. Each codec has one source shared by WebRTC and Direct; the final
subscriber releases that source. Changing transport within a codec preserves
the encoder, and teardown of a previously selected codec cannot stop its successor.
Screenshots use fresh independent frames instead of stealing streaming AUs;
encoded snapshots require GStreamer libav decoders. HDMI audio uses
`hw:AMLAUGESOUND,0`; microphone playback requires `aplay` and
`plughw:UAC2Gadget,0`.

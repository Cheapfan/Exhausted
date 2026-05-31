## What to build

Real hardware integration and deployment: ALSA audio capture, V4L2 video capture, systemd supervision, and a Raspberry Pi provisioning script.

**ALSA AudioSource** (`internal/audio/alsa.go`):
- Implements `AudioSource` interface
- Opens ALSA device (`hw:0,0` or configurable) via `goalsa` or raw `ioctl`
- Captures 44.1kHz, 16-bit signed integer, mono
- Returns `[]int16` samples matching the 1024-sample expected chunk size
- Configurable device name, sample rate, buffer size

**V4L2 VideoSource** (`internal/video/v4l2.go`):
- Implements `VideoSource` interface
- Opens `/dev/video0` (or configurable)
- Requests MJPEG format at 1280×720, 30fps
- Returns `[]byte` JPEG frames
- Configurable device path, resolution, fps, pixel format

**Integration into binaries:**
- NoiseComplaint: `--source` flag with values `mock|alsa` — selects between WAV file and live ALSA
- PanicClipper: `--source` flag with values `mock|v4l2` — selects between JPEG directory and live camera
- Default to `mock` when no flag provided (safe for dev)

**systemd service files** (`deploy/edge/systemd/`):
- `exhausted-noisecomplaint.service`
- `exhausted-panicclipper.service`
- `exhausted-gogopher.service`
- Each: `Type=simple`, `Restart=always`, `RestartSec=5`, logging to journald
- Dependencies: gogopher starts first, noisecomplaint+panicclipper start after (but not hard-dep — they should retry connection)

**Setup script** (`deploy/edge/setup.sh`):
- Install Go toolchain
- Install FFmpeg with V4L2 support
- Build Go binaries
- Copy binaries and config to `/opt/exhausted/`
- Copy systemd unit files to `/etc/systemd/system/`
- `systemctl enable` + `systemctl start`
- Create tmpfs mount at `/tmpfs/` (512 MB)

**Config file** (`/opt/exhausted/config.toml`):
- `gps.lat`, `gps.lon` — hardcoded deployment location
- `audio.device`, `audio.threshold_db`, `audio.ratio_threshold`
- `video.device`, `video.fps`, `video.pre_trigger_secs`, `video.post_trigger_secs`, `video.bitrate`
- `gogopher.listen_addr`, `gogopher.panicclipper_url`
- `cloud.url`, `cloud.auth_token`
- `sync.base_delay`, `sync.max_delay`

## Acceptance criteria

- [ ] ALSA AudioSource captures real microphone input on Pi
- [ ] V4L2 VideoSource captures real camera frames on Pi
- [ ] NoiseComplaint runs with `--source=alsa` and triggers on real loud noises
- [ ] PanicClipper runs with `--source=v4l2` and buffers real camera frames
- [ ] All 3 systemd services start, stop, and restart correctly
- [ ] `systemctl restart exhausteds-panicclipper` — service comes back, buffer rebuilds
- [ ] Config file values are loaded and override defaults
- [ ] `setup.sh` is idempotent (safe to run multiple times)
- [ ] tmpfs is mounted at `/tmpfs/` with 512 MB limit

## Blocked by

- #003 — NoiseComplaint trigger loop (to have ALSA plugged in)
- #005 — PanicClipper (to have V4L2 plugged in)

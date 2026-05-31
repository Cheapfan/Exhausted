## What to build

PanicClipper — the video buffering and encoding binary. Maintains a rolling MJPEG ring buffer, snapshots on demand, and encodes via FFmpeg hardware acceleration.

- `cmd/panicclipper/main.go` — application entry point
- `VideoSource` mock that reads JPEG files from a directory (numbered like `frame_000001.jpg`)
- `RingBuffer` in `internal/video/ringbuffer.go`:
  - Configurable max frames (default 150)
  - `Push(frame []byte)` — adds JPEG to buffer, evicts oldest if full
  - `Snapshot() ([][]byte, int)` — returns all current frames + the index where pre ends and post begins
  - Thread-safe with `sync.RWMutex`
- `Encoder` in `internal/video/encoder.go`:
  - `Encode(frames [][]byte, fps int, outputPath string) error`
  - Writes frames to temp dir, runs: `ffmpeg -y -f image2 -framerate N -i %05d.jpg -c:v h264_v4l2m2m -b:v 1200k output.mp4`
  - Cleans up temp dir on success/failure
- HTTP server on a localhost port:
  - `POST /dump` — triggers snapshot, captures 2 seconds (2×fps more frames), encodes, returns `{"file_path": "/tmpfs/output.mp4"}`
- Config: `--fps` (default 30), `--pre-trigger-secs` (default 3), `--post-trigger-secs` (default 2), `--bitrate` (default 1200k), `--mock-dir` (path to JPEG directory for mock source)

**Tests:**
- RingBuffer push/snapshot/wrap-around with known frame counts
- Encoder with small frame set (5 frames, low resolution for speed)
- HTTP handler with httptest

## Acceptance criteria

- [ ] RingBuffer pushes frames up to max, evicts oldest at boundary
- [ ] RingBuffer snapshot returns correct frames in order
- [ ] RingBuffer is thread-safe (test concurrent push + snapshot)
- [ ] Encoder produces a valid .mp4 file at the specified path
- [ ] `POST /dump` returns JSON with `file_path`
- [ ] After `/dump`, the buffer continues accepting new frames
- [ ] All constants are overridable via CLI flags

## Blocked by

- #001 — VideoSource interface definition

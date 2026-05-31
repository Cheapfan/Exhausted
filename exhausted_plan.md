# **Exhausted — System Design (Post-Grill)**

## Architecture
- **Edge:** Go (NoiseComplaint, PanicClipper, GoGopher) — 3 separate binaries under systemd
- **Cloud:** Python (Ingestor + Processor), PostgreSQL 16 + PostGIS
- **IPC:** Localhost HTTP between edge binaries

## Audio (NoiseComplaint)
- ALSA via Go bindings, USB/3.5mm mic, 44.1kHz 16-bit mono
- Configurable 85 dB threshold
- Energy ratio discriminant: `low-band(100–250Hz) / mid-band(250–500Hz)` — >60% = modified exhaust
- Sends POST to GoGopher `/api/v1/internal/trigger`

## Video (PanicClipper)
- USB camera, V4L2 capture, MJPEG frames, 720p/30fps (60fps configurable for Pi 5)
- MJPEG ring buffer (~3.75 MB for 150 frames) — NOT raw BGR24
- Long-lived daemon: maintains buffer, `POST /dump` snapshots + continues capturing 2s post-trigger
- FFmpeg subprocess with `h264_v4l2m2m` hardware encoding
- All constants (fps, pre/post duration, bitrate) are configurable

## GoGopher (Coordination)
- Loopback HTTP server on `127.0.0.1:8080`
- SQLite outbox (WAL mode) with `sync_status IN (0,2,4)`:
  - `0` = Pending, `1` = Active upload, `2` = Error, `3` = Terminal Failure, `4` = Awaiting Video
- SyncWorker picks pending records → calls PanicClipper `/dump` → updates file_path → uploads
- Infinite retry with exponential backoff, 30-minute max delay ceiling (no maxRetries)

## Cloud (Python)
- **Ingestor** (FastAPI): Receives multipart upload → 202 Accepted → writes to DB
- **Processor** (background worker): Polls DB for unprocessed clips → YOLOv8m INT8 (ONNX Runtime CPU) → vectorized Gaussian blur → updates DB
- 800ms processing SLO per clip, not per request
- Nginx reverse proxy in front

## Database
- Single `violations` table with JSONB for vehicle detections
- PostGIS `GEOGRAPHY(Point, 4326)` on location

## Development
- Interface-driven Go: `AudioSource`, `VideoSource` with ALSA/V4L2 + file-based mocks
- Develop on laptop with mocks, deploy with systemd on Pi 4
- Geolocation from hardcoded config file

## Project Structure
```
exhausted/
├── edge/
│   ├── cmd/
│   │   ├── noisecomplaint/main.go
│   │   ├── panicclipper/main.go
│   │   └── gogopher/main.go
│   ├── internal/
│   │   ├── audio/   (capture, rms, fft, discriminant)
│   │   ├── video/   (capture, ringbuffer, encoder)
│   │   ├── storage/ (sqlite outbox)
│   │   └── sync/    (upload worker)
│   └── go.mod
├── cloud/
│   ├── ingestor/main.py
│   ├── processor/inference.py
│   ├── processor/anonymizer.py
│   └── db/schema.sql
└── deploy/
    ├── edge/systemd/
    └── cloud/docker-compose.yml
```

# PRD: Exhausted — Acoustic Traffic Violation Detection Framework

## Problem Statement

Urban environments suffer from pervasive acoustic pollution, with modified vehicle exhausts as a primary contributor to degraded public health and ambient noise standard infractions. Current enforcement approaches are inadequate:

- **Manual enforcement** requires physical deployment of traffic officers, exposes them to environmental hazards, and produces inconsistent, subjective enforcement.
- **Continuous video streaming** systems upload high-definition footage to the cloud 24/7, incurring prohibitive cellular bandwidth costs on metered 4G/5G networks, consuming massive cloud storage, and recording thousands of non-violating citizens — creating systemic privacy liabilities.

Existing solutions are either too expensive to deploy at scale or violate privacy by continuously recording public spaces. There is no low-cost, event-driven system that captures evidence only when an acoustic violation is mathematically proven and automatically processes it for enforcement.

## Solution

**Exhausted** is an automated, event-driven, edge-to-cloud IoT framework deployed on Raspberry Pi 4 hardware along arterial roadways. It:

1. **Listens passively** with a USB microphone, running real-time FFT analysis to detect modified exhaust signatures (100–250 Hz low-band energy concentration at >85 dB).
2. **Captures video only on trigger** — a rolling MJPEG frame buffer keeps 3 seconds of pre-trigger footage in ~3.75 MB of RAM. On violation, it records 2 more seconds, then compresses the 5-second clip via hardware-accelerated H.264 encoding.
3. **Uploads over metered cellular** using a transactional outbox (SQLite WAL) with infinite retry and exponential backoff capped at 30 minutes.
4. **Processes in the cloud** — YOLOv8m INT8 via ONNX Runtime CPU identifies vehicles, license plates, and faces. A vectorized Gaussian blur anonymizes privacy-sensitive regions before persistent storage in PostgreSQL with PostGIS.

The system is built in **Go on the edge** (for concurrency, compiled binaries, systems-level learning) and **Python in the cloud** (for AI/ML ecosystem access).

## User Stories

1. As a roadside edge node, I want to continuously capture 44.1kHz 16-bit mono audio via ALSA, so that I can detect loud exhaust events in real time.
2. As a roadside edge node, I want to compute RMS energy on 1024-sample windows of audio, so that I can calculate instantaneous decibel levels.
3. As a roadside edge node, I want to apply a Hanning window and FFT when decibel levels exceed 85 dB, so that I can analyze the frequency domain of the sound.
4. As a roadside edge node, I want to compute an energy ratio between the low-frequency band (100–250 Hz) and mid-frequency band (250–500 Hz), so that I can discriminate modified exhausts from broad-spectrum urban noise (diesel trucks, construction, sirens).
5. As a roadside edge node, I want to fire a trigger event (POST to localhost) only when the low-band energy ratio exceeds 60%, so that I minimize false positives.
6. As a roadside edge node, I want to maintain a rolling ring buffer of MJPEG-compressed camera frames at 720p/30fps, so that I can reconstruct pre-trigger footage without consuming 400 MB of RAM for raw frames.
7. As a roadside edge node, I want my video capture daemon to run as a long-lived process that never exits, so that I avoid camera re-initialization delays after each violation.
8. As a roadside edge node, I want to snap the ring buffer and continue capturing 2 seconds of post-trigger footage on demand, so that I capture both the vehicle approaching and passing.
9. As a roadside edge node, I want to encode the 5-second frame sequence into H.264 via FFmpeg with hardware acceleration (h264_v4l2m2m), so that the clip stays under 1.5 MB for metered cellular upload.
10. As a roadside edge node, I want an outbox queue stored in SQLite with WAL mode, so that violation records survive power failure or unexpected reboot.
11. As a roadside edge node, I want the trigger event to create an outbox record in "Awaiting Video" status (4), so that the sync worker knows to request video before uploading.
12. As a roadside edge node, I want the sync worker to call the video daemon's dump endpoint, wait for the encoded clip, and update the outbox file path, so that upload can proceed.
13. As a roadside edge node, I want the sync worker to upload the clip and metadata via HTTPS multipart POST to the cloud, so that the evidence reaches the processing backend.
14. As a roadside edge node, I want the sync worker to retry infinitely with exponential backoff capped at 30 minutes, so that events are never permanently lost due to temporary cellular disconnection.
15. As a roadside edge node, I want the sync worker to purge the file and outbox record on successful upload (HTTP 200 OK), so that tmpfs disk space is reclaimed.
16. As a roadside edge node, I want to configure geolocation (lat/lon) from a static config file, so that I don't need GPS hardware for a stationary deployment.
17. As a cloud ingestor, I want to accept multipart uploads via a REST API and return 202 Accepted immediately, so that edge nodes don't time out on shaky cellular links.
18. As a cloud processor, I want to poll PostgreSQL for unprocessed violation records, so that I can process clips without requiring Redis or a message queue.
19. As a cloud processor, I want to run YOLOv8m INT8 via ONNX Runtime on CPU with AVX2/OpenMP, so that I detect vehicles, license plates, and faces without a GPU.
20. As a cloud processor, I want to extract bounding boxes for license plates and faces in each frame and apply a multi-threaded Gaussian blur, so that privacy is protected before storage.
21. As a system operator, I want all violation data stored in PostgreSQL 16 with PostGIS, so that I can query by geospatial area, time range, vehicle class, or detection confidence.
22. As a system operator, I want to retrieve the anonymized video clip for any violation from cloud storage, so that I can manually review evidence.
23. As a developer, I want to develop and test the edge Go binaries on my laptop using mock audio/video sources, so that I don't need a Raspberry Pi with peripherals for every code change.
24. As a developer, I want the edge binaries to be supervised by systemd, so that they restart automatically on crash with proper logging.
25. As a developer, I want the cloud services to run in Docker Compose, so that deployment is reproducible across environments.
26. As a developer, I want unit tests for the audio processing pipeline (RMS, FFT, energy ratio) with known sample inputs, so that the detection algorithm is verified.
27. As a developer, I want unit tests for the ring buffer state machine (empty, full, wrap-around, snapshot during capture), so that frame integrity is guaranteed.
28. As a developer, I want integration tests for the SQLite outbox store (create, status transitions, fetch pending, delete), so that transactional correctness is proven.
29. As a developer, I want unit tests for the exponential backoff calculator with various attempt counts and jitter, so that the retry timing is predictable.
30. As a developer, I want integration tests for the upload sync worker with a mock HTTP server, so that upload success/failure/retry flows are verified.
31. As a developer, I want automated tests for the YOLOv8 inference pipeline and Gaussian blur using sample test images, so that cloud processing quality is validated in CI.

## Implementation Decisions

### Language and Architecture
- **Edge:** Go (all three binaries — NoiseComplaint, PanicClipper, GoGopher). One compiled binary per daemon, no Python runtime on Pi.
- **Cloud:** Python (FastAPI Ingestor + background Processor). Native access to NumPy, OpenCV, ONNX Runtime, scikit-image.
- **IPC:** Localhost HTTP between edge binaries. NoiseComplaint → GoGopher via POST `/api/v1/internal/trigger`. GoGopher → PanicClipper via POST `/dump`.

### Audio Capture and Processing
- **Hardware:** USB microphone or 3.5mm electret mic via ALSA. 44.1 kHz, 16-bit signed integer PCM, mono.
- **Detection pipeline:** 1024-sample windows → RMS → dB conversion. If >85 dB (configurable): Hanning window → FFT → energy ratio of 100–250 Hz band vs 250–500 Hz band. Only trigger if low-band ratio >60%.
- **Threshold and frequency bands are configurable** at startup via config file or env vars.

### Video Capture and Encoding
- **Hardware:** USB camera via V4L2. Target 720p/30fps MJPEG. 60fps support is parameterized for future Pi 5 migration.
- **Ring buffer:** Stores MJPEG-compressed JPEG frames (~25 KB each). 150 frames (~3.75 MB total) for 3s pre-trigger + 2s post-trigger allocation.
- **Encoding:** FFmpeg subprocess with `h264_v4l2m2m` hardware acceleration. Bitrate target 1200 kbps. All parameters (fps, pre/post duration, bitrate) are configurable constants.
- **Daemon lifecycle:** Long-lived process. Never exits between dumps.

### GoGopher Coordination
- **Trigger endpoint:** `POST /api/v1/internal/trigger` on `127.0.0.1:8080`. Accepts JSON payload. Returns 202 Accepted.
- **Outbox database:** SQLite with WAL mode. Table `violation_outbox` with primary key `event_id` (UUIDv4).
  - Statuses: 0=Pending, 1=Active (uploading), 2=Error, 3=Terminal Failure, 4=Awaiting Video
- **Sync flow:** Audio triggers → create row (status=4) → sync worker picks it → calls PanicClipper `/dump` → updates file_path and status to 0 → uploads.
- **Upload:** HTTPS multipart POST with TLS 1.3. Auth via `X-Node-Auth-Token` header.
- **Retry:** Infinite retries. Exponential backoff: `min(maxDelay, baseDelay × 2^attempt + 50% jitter)`. `baseDelay = 2s`, `maxDelay = 30min`.

### Cloud Processing (Silencer)
- **Ingestor (FastAPI):** `POST /api/v1/upload` receives multipart form with video file + metadata fields. Returns 202. Writes record to `violations` table with `processing_status = 0`. Video stored to local disk or S3-compatible storage.
- **Processor (background worker):** Polls `violations` table for `processing_status = 0` every 5 seconds.
- **Inference:** ONNX Runtime session with `CPUExecutionProvider`, `intra_op_num_threads = cpu_count()`, full graph optimization. INT8 quantized YOLOv8m model. Input: 640×640 RGB. Output: vehicle/plate/face bounding boxes.
- **Anonymization:** Vectorized Gaussian blur. Kernel size = 25% of bounding box dimension, forced to odd integer. Applied per-frame per-bounding-box via OpenCV.
- **Storage:** After processing, writes update the same PostgreSQL row (inference results as JSONB, processing time, status = 1).

### Database Schema (PostgreSQL 16 + PostGIS)

```sql
CREATE TABLE violations (
    event_id         UUID PRIMARY KEY,
    recorded_at      TIMESTAMPTZ NOT NULL,
    ingested_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    peak_decibel     REAL NOT NULL,
    frequency_hz     REAL NOT NULL,
    location         GEOGRAPHY(Point, 4326),
    video_path       TEXT,
    clip_size_bytes  BIGINT,

    vehicle_count    INT DEFAULT 0,
    vehicles         JSONB DEFAULT '[]',
    has_plate        BOOLEAN DEFAULT FALSE,
    plate_bboxes     JSONB DEFAULT '[]',
    has_face         BOOLEAN DEFAULT FALSE,
    face_bboxes      JSONB DEFAULT '[]',
    blur_kernel_used JSONB DEFAULT '[]',

    processing_time_ms INT,
    processing_status INT DEFAULT 0  -- 0=pending, 1=done, 2=error
);
```

### Go Interfaces (for testability)

```go
type AudioSource interface {
    ReadSample() ([]int16, error)
    Close() error
}

type VideoSource interface {
    ReadFrame() ([]byte, error)  // Returns JPEG bytes
    Close() error
}
```

ALSA and V4L2 implementations behind these interfaces. Mock implementations read from files for laptop development.

### Project Structure

```
exhausted/
├── edge/                          # Go edge binaries
│   ├── cmd/
│   │   ├── noisecomplaint/main.go
│   │   ├── panicclipper/main.go
│   │   └── gogopher/main.go
│   ├── internal/
│   │   ├── audio/                 # capture, rms, fft, discriminant
│   │   ├── video/                 # capture, ringbuffer, encoder
│   │   ├── storage/               # sqlite outbox
│   │   └── sync/                  # upload worker with backoff
│   ├── go.mod
│   └── go.sum
├── cloud/                         # Python cloud services
│   ├── ingestor/main.py           # FastAPI
│   ├── processor/
│   │   ├── inference.py           # ONNX Runtime
│   │   └── anonymizer.py          # Gaussian blur
│   └── db/schema.sql              # PostgreSQL schema
├── deploy/
│   ├── edge/systemd/              # .service files
│   └── cloud/docker-compose.yml
├── PRD.md
└── exhausted_plan.md
```

## Testing Decisions

**What makes a good test:** Test external behavior, not implementation details. Each module's public interface is the contract. Tests should use real implementations of dependencies where practical (in-memory SQLite, mock HTTP servers, file-based mock sources), never mock internal internals.

**Modules to be tested (all of them):**

| Module | Test Type | Approach |
|---|---|---|
| Audio processing (RMS, FFT, energy ratio) | Unit | Feed known sample arrays (sine waves, mixed frequencies), assert dB values, FFT peaks, energy ratios |
| Ring buffer | Unit | Push frames, snapshot, drain, test wrap-around at boundary, test concurrent read/write |
| SQLite outbox store | Integration | In-memory SQLite, test CRUD, status transitions (0→4→0→1→delete), edge cases (empty table) |
| Exponential backoff | Unit | Assert calculated delays for attempt 0–20, assert jitter is within 0–50% range, assert maxDelay clamping |
| Upload sync worker | Integration | Mock HTTP server returning 200/500/timeout, assert status transitions and retry calls |
| Encoder (FFmpeg) | Integration | Write known MJPEG frames, run encoder, assert .mp4 output exists, assert file size threshold |
| Audio/Video capture mocks | Unit | Verify mock sources produce expected sample values and can simulate EOF/errors |
| YOLOv8 inference pipeline | Integration | Load known test image, run inference, assert expected class/bbox output shape |
| Gaussian blur anonymizer | Integration | Create test frame with synthetic "plate" region, blur, assert pixel variance in region |

**Prior art:** This is a greenfield project — no prior tests. The testing approach follows standard Go table-driven tests and Python pytest conventions.

## Out of Scope

- **License plate OCR (ALPR):** Plate text recognition is not in scope. This PRD covers vehicle *detection* and privacy *anonymization* only.
- **GPU acceleration:** All inference runs on CPU via ONNX Runtime with AVX2/OpenMP. GPU support is out of scope.
- **Mobile app / web dashboard:** No frontend. Data is stored in PostgreSQL for future frontend consumption.
- **Multi-camera synchronization:** Each Pi has one camera. No cross-Pi event correlation.
- **Real-time audio streaming to cloud:** All processing is edge-first. No raw audio leaves the Pi.
- **Scalability / load balancing:** Single Pi → single cloud instance. Horizontal scaling is out of scope.
- **Hardware manufacturing:** The plan uses off-the-shelf Raspberry Pi 4, USB camera, USB mic. No custom PCB or enclosure design.

## Further Notes

- All detection parameters (dB threshold, frequency bands, energy ratio threshold, fps, pre/post trigger duration, bitrate, backoff base/max delay) are **configurable** — not hardcoded. Use a TOML or JSON config file read at startup.
- The project was conceived as a personal portfolio piece to demonstrate Go systems programming (ALSA, V4L2, SQLite, HTTP, backoff) and Python ML deployment (ONNX Runtime, OpenMP optimization, OpenCV).
- Development follows an interface-driven approach: write mocks, build logic on laptop, deploy to Pi for hardware integration tests.
- The 100-250 Hz low-band and 250-500 Hz mid-band are initial defaults. Real-world testing may require adjustment. The energy ratio discriminant is designed to be recalibrated without code changes.

## What to build

NoiseComplaint — the audio sensing binary that reads audio from a mock source (WAV file), runs the AudioProcessor pipeline, and fires HTTP triggers to GoGopher.

- `cmd/noisecomplaint/main.go` — application entry point
- AudioSource mock that reads frames from a WAV file (fixed 44.1kHz, 16-bit mono)
- Processing loop: reads 1024-sample chunks → AudioProcessor → if `Classify()` returns true → HTTP POST to GoGopher
- Config via CLI flags or env vars:
  - `--wav` path to WAV file (mock audio source)
  - `--gogopher-url` default `http://127.0.0.1:8080/api/v1/internal/trigger`
  - `--db-threshold` default 85.0
  - `--ratio-threshold` default 0.6
- HTTP client posts JSON trigger payload to GoGopher
- Logs each chunk's dB level, dominant frequency, energy ratio, and whether it triggered

**End-to-end test:**
- Start GoGopher (from #003) with SQLite outbox
- Run NoiseComplaint with a WAV file containing a synthetic modified exhaust (150 Hz sine at 94 dB)
- Verify a violation record appears in GoGopher's SQLite outbox

## Acceptance criteria

- [ ] NoiseComplaint reads 1024-sample chunks from a WAV file
- [ ] Each chunk runs through AudioProcessor and logs dB + energy ratio
- [ ] When dB > threshold AND ratio > threshold, HTTP POST is sent to GoGopher
- [ ] When below threshold, no HTTP POST is sent
- [ ] End-to-end: synthetic violation WAV → GoGopher SQLite has a pending record
- [ ] Silent WAV file → no records created
- [ ] Broad-spectrum noise WAV (diesel truck sim) → no records created (tests ratio discriminant)

## Blocked by

- #001 — AudioProcessor library
- #003 — GoGopher HTTP server + SQLite outbox

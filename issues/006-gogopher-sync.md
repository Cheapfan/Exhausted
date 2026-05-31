## What to build

The GoGopher synchronization flow: picking outbox records, calling PanicClipper for video, and uploading to the cloud with exponential backoff.

**SyncWorker** additions to `internal/sync/`:
- `ProcessSyncQueue(ctx)` — main loop, selects records from outbox
- For status=4 records: calls PanicClipper `POST /dump`, gets file_path, updates record to status=0
- For status=0/2 records: calls cloud Ingestor upload
- `CalculateBackoff(attempt int) time.Duration` — `min(maxDelay, baseDelay × 2^attempt + 50% jitter)` with `baseDelay=2s`, `maxDelay=30min`
- Infinite retry — permanently wait at maxDelay ceiling, never terminal-fail

**Upload client:**
- Multipart form construction: video file + metadata fields
- `POST` to configured cloud URL
- Header: `X-Node-Auth-Token`
- On success (200): delete outbox record, delete file from disk
- On failure: update retry_count, status=2, last_attempt, sleep backoff

**Config:** loaded from CLI flags or env vars:
- `--panicclipper-url` default `http://127.0.0.1:9090/dump`
- `--cloud-url` cloud Ingestor endpoint
- `--node-token` auth token
- `--base-delay` default 2s
- `--max-delay` default 30m

**Tests:**
- Backoff calculator: assert exponential growth, jitter range, max clamping
- Upload flow: mock HTTP server returning 200, 500, timeout — assert outbox status transitions and retry count increments
- PanicClipper integration: mock HTTP server for `/dump` — assert file_path updated in outbox

## Acceptance criteria

- [ ] `CalculateBackoff(0)` ≈ 2–3s
- [ ] `CalculateBackoff(14)` clamped to 30m
- [ ] Jitter is always between 0% and 50% of calculated delay
- [ ] Upload success → outbox record deleted, file deleted from disk
- [ ] Upload failure → retry_count incremented, status=2, last_attempt updated
- [ ] After hitting maxDelay, all subsequent delays are exactly maxDelay
- [ ] Status=4 records cause HTTP call to PanicClipper then transition to status=0
- [ ] PanicClipper failure does not crash the sync loop

## Blocked by

- #003 — GoGopher SQLite outbox store
- #005 — PanicClipper with `/dump` endpoint

## What to build

The GoGopher daemon core: SQLite outbox store and the localhost HTTP trigger server.

**SQLite outbox store** (`internal/storage/`):
- `OutboxStore` struct with `*sql.DB`
- `NewOutboxStore(dbPath string) (*OutboxStore, error)` — opens DB, runs migration
- `Create(eventID, timestamp string, peakDB, freqHz, lat, lon float64) error` — inserts with status=4
- `GetPending() (*OutboxRecord, error)` — fetches next record with status 0 or 2
- `GetAwaitingVideo() (*OutboxRecord, error)` — fetches next record with status 4
- `MarkActive(eventID string) error` — status 0→1
- `MarkAwaitingVideo(eventID string) error` — status 4 (already default on create)
- `MarkComplete(eventID string) error` — delete the record (success)
- `MarkError(eventID string) error` — status→2
- `MarkTerminal(eventID string) error` — status→3
- Schema: `violation_outbox` table with UUID PK, TIMESTAMP, REALs, VARCHAR(255) file_path, REAL lat/lon, INT sync_status, INT retry_count, TIMESTAMP last_attempt

**HTTP server** (in `cmd/gogopher/main.go`):
- Listens on `127.0.0.1:8080`
- `POST /api/v1/internal/trigger` — accepts JSON `{"timestamp","peak_decibel","frequency_hz","gps":{"lat","lon"}}`
- Validation: timestamps parseable, decibel ≥ 0, frequency ≥ 0, lat/lon in valid ranges
- Returns `202 Accepted` on success
- Returns `400 Bad Request` on invalid payload
- Writes to outbox via `OutboxStore.Create()`

**Tests:**
- SQLite store with in-memory database
- HTTP server with `httptest.Server`
- Cover all status transitions

## Acceptance criteria

- [ ] `OutboxStore` CRUD operations work with in-memory SQLite
- [ ] `POST /trigger` with valid payload returns 202
- [ ] `POST /trigger` with invalid JSON returns 400
- [ ] `POST /trigger` with out-of-range values returns 400
- [ ] After `POST /trigger`, a row exists in SQLite with status=4
- [ ] `GetPending` returns rows with status 0 or 2 only
- [ ] `GetAwaitingVideo` returns rows with status 4 only
- [ ] Status transitions follow expected flow (4→0→1→delete)
- [ ] All tables created on first `NewOutboxStore` call

## Blocked by

- #001 — Go project scaffold and interfaces

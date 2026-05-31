## What to build

The cloud-side upload receiver — a FastAPI service that accepts multipart video uploads from edge nodes and stores violation records in PostgreSQL.

**Ingestor (FastAPI):**
- `POST /api/v1/upload` — accepts multipart form with fields:
  - `video_payload` — the .mp4 file
  - `event_id` — UUIDv4 from edge
  - `recorded_at` — ISO 8601 timestamp (added during implementation — PRD schema requires it)
  - `peak_decibel` — float
  - `frequency_hz` — float
  - `latitude` — float
  - `longitude` — float
  - `clip_size_bytes` — int64
- Returns `202 Accepted` immediately
- Writes a row to the `violations` table with `processing_status = 0`

**PostgreSQL schema:**
- PostGIS extension
- `violations` table as defined in PRD (UUID PK, TIMESTAMPTZ, REAL, GEOGRAPHY, TEXT, BIGINT, JSONB columns, processing_status INT)

**Docker Compose:**
- `cloud/docker-compose.yml` with `db` (postgis/postgis:16-3.4) and `ingestor` (FastAPI) services
- Volume-mounted data directory

## Implementation

### Project structure

```
cloud/
├── docker-compose.yml               # db + ingestor services
├── requirements.txt                 # Python deps (fastapi, asyncpg, uvicorn, etc.)
├── db/
│   └── schema.sql                   # Idempotent violations table + PostGIS
└── ingestor/
    ├── Dockerfile                   # Python 3.12-slim, copies db/ + ingestor/
    ├── main.py                      # FastAPI app (2 endpoints + lifespan DB init)
    └── tests/
        └── test_ingestor.py         # 9 tests with mocked DB
```

### Database schema (`cloud/db/schema.sql`)

```sql
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS violations (
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
    processing_status INT DEFAULT 0
);
```

Both `CREATE EXTENSION` and `CREATE TABLE` use `IF NOT EXISTS` — running this multiple times is safe (idempotent).

The schema is run in two places:
1. **Docker Compose init**: mounted as `db:/docker-entrypoint-initdb.d/01-schema.sql` so PostGIS creates the table on first start
2. **Application startup**: the FastAPI lifespan reads and executes `schema.sql` after connecting to the DB pool — this covers restarts and non-Docker use

### FastAPI endpoint (`cloud/ingestor/main.py`)

**`POST /api/v1/upload`**

The endpoint uses FastAPI's `File` and `Form` dependency injection — no manual `request.form()` parsing.

Request flow:
1. **File type validation** — checks `filename.endswith(".mp4")`. Returns 400 for any other extension.
2. **UUID validation** — parses `event_id` via `uuid.UUID()`. Returns 422 on invalid.
3. **Timestamp validation** — parses `recorded_at` via `datetime.fromisoformat()`. Returns 422 on invalid (handles `Z` suffix and `+00:00` offsets on Python 3.12).
4. **FastAPI built-in validation** — type annotations on `Form()` params enforce `float`/`int` coercion. Missing fields and type mismatches return 422 automatically.
5. **Video saved to disk** — written to `{VIDEO_DIR}/{event_id}.mp4`. Directory created if absent.
6. **DB insert** — `asyncpg` parameterized INSERT with PostGIS `ST_MakePoint`/`ST_SetSRID` for the location column.
7. **Returns 202** — `{"status": "accepted", "event_id": "..."}`.

**`GET /health`** — returns `{"status": "ok"}`. Used for Docker Compose health checks and curl-verifiable startup.

### Database connection & dependency injection

Two-layer design for testability:

```
Application layer → dependency override layer → real PostgreSQL
                                               → MockDB (in tests)
```

- **`Database` class** wraps `asyncpg.Pool` with an `insert_violation()` method using parameterized queries.
- **`get_db()` function** is a FastAPI `Depends` callable. If the pool is `None` (startup failed), returns 503.
- **`lifespan` context manager** creates the pool and runs schema.sql on startup, closes the pool on shutdown. If DB is unreachable, it logs a warning and continues — the app still starts and serves `/health`.
- **In tests**, `app.dependency_overrides[get_db]` replaces it with `MockDB`, which records calls for assertion. No real PostgreSQL required.

### Docker Compose setup (`cloud/docker-compose.yml`)

```yaml
services:
  db:
    image: postgis/postgis:16-3.4
    healthcheck: pg_isready
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./db/schema.sql:/docker-entrypoint-initdb.d/01-schema.sql
  ingestor:
    build: .
    depends_on:
      db: condition: service_healthy
    environment:
      DATABASE_URL: postgresql://exhausted:exhausted@db:5432/exhausted
      VIDEO_DIR: /data/videos
```

- The ingestor waits for `db` to be healthy (responding to `pg_isready`) before starting.
- `DATABASE_URL` is set to use the Docker network hostname `db`.
- Video files persist in a named volume `video_data`.

### Test coverage (9 tests, all passing)

All tests use `TestClient` with `dependency_overrides` — no real PostgreSQL needed.

| Test | What it verifies |
|---|---|
| `test_202_on_valid_upload` | Complete valid request returns 202 with `status: "accepted"` |
| `test_422_when_event_id_missing` | Missing required form field returns 422 |
| `test_422_when_peak_decibel_missing` | Missing float field returns 422 |
| `test_422_on_invalid_uuid` | Non-UUID `event_id` returns 422 |
| `test_422_on_invalid_timestamp` | Non-ISO8601 `recorded_at` returns 422 |
| `test_400_on_non_mp4_file` | `.avi` file returns 400 with "mp4" in error message |
| `test_400_on_missing_extension` | File without `.mp4` extension returns 400 |
| `test_422_on_invalid_int` | `clip_size_bytes="not-a-number"` returns 422 (FastAPI type coercion) |
| `test_health` | `GET /health` returns 200 with `status: "ok"` |

### Important design decisions

1. **`recorded_at` added to API.** The original issue spec omitted it, but the PRD schema requires `recorded_at TIMESTAMPTZ NOT NULL`. The edge node knows when the violation happened; the server should preserve that timestamp rather than using only `ingested_at`.

2. **`asyncpg` (async) for DB access.** FastAPI is async-first. Using `asyncpg` instead of `psycopg2` avoids running DB queries in a thread pool. The pool is created once at startup with `min_size=1, max_size=5`.

3. **PostGIS location via ST_MakePoint.** The insert uses `ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)` — note the argument order: `ST_MakePoint` takes `(x, y)` = `(lon, lat)`, not `(lat, lon)`. This is a common gotcha with PostGIS.

4. **Graceful DB failure.** If PostgreSQL is unreachable at startup, the lifespan logs a warning and the app still serves `/health`. The `get_db` dependency returns 503 for upload requests. This lets the container start and report its status rather than crash-looping.

5. **Video stored to local disk.** `VIDEO_DIR` defaults to `/data/videos`. In Docker Compose, this is a named volume. S3 support would be a future enhancement.

6. **IDempotent schema.** Both `CREATE EXTENSION` and `CREATE TABLE` use `IF NOT EXISTS`. The schema is also mounted as a Docker init script — if the container restarts, the schema won't error on re-run.

## Acceptance criteria

- [ ] `docker compose up` starts both PostGIS and FastAPI
- [x] `POST /api/v1/upload` with a test multipart request returns 202 (verified by unit test)
- [ ] Violation record appears in PostgreSQL after upload (requires Docker to verify end-to-end; unit test verifies the `insert_violation` mock is called with correct args)
- [x] Missing required fields return 422 (verified by tests for event_id, peak_decibel, clip_size_bytes)
- [x] Invalid file type returns 400 (verified by tests for .avi extension and missing extension)
- [x] `schema.sql` is idempotent (uses `IF NOT EXISTS` on both EXTENSION and TABLE)
- [ ] FastAPI auto-docs available at `/docs` (requires running the app)

## Blocked by

None — can start immediately.

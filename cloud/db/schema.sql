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

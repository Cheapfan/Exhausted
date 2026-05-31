import os
import uuid
from contextlib import asynccontextmanager
from datetime import datetime, timezone

import aiofiles
import asyncpg
from fastapi import FastAPI, File, Form, UploadFile, HTTPException, Depends


DATABASE_URL = os.environ.get(
    "DATABASE_URL",
    "postgresql://exhausted:exhausted@localhost:5432/exhausted",
)
VIDEO_DIR = os.environ.get("VIDEO_DIR", "/data/videos")


class Database:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self.pool = pool

    async def insert_violation(
        self,
        event_id: uuid.UUID,
        recorded_at: datetime,
        peak_decibel: float,
        frequency_hz: float,
        latitude: float,
        longitude: float,
        video_path: str,
        clip_size_bytes: int,
    ) -> None:
        async with self.pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO violations
                    (event_id, recorded_at, peak_decibel, frequency_hz,
                     location, video_path, clip_size_bytes)
                VALUES ($1, $2, $3, $4,
                        ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8)
                """,
                event_id,
                recorded_at,
                peak_decibel,
                frequency_hz,
                longitude,
                latitude,
                video_path,
                clip_size_bytes,
            )


_db_pool: asyncpg.Pool | None = None


async def get_db() -> Database:
    if _db_pool is None:
        raise HTTPException(503, "Database not available")
    return Database(_db_pool)


schema_path = os.path.join(os.path.dirname(__file__), "..", "db", "schema.sql")


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _db_pool
    pool = None
    started = False
    try:
        pool = await asyncpg.create_pool(DATABASE_URL, min_size=1, max_size=5)
        async with pool.acquire() as conn:
            with open(schema_path) as f:
                await conn.execute(f.read())
        _db_pool = pool
        started = True
    except Exception as e:
        print(f"WARNING: could not connect to database: {e}")
    yield
    if started and _db_pool is not None:
        await _db_pool.close()
        _db_pool = None


app = FastAPI(title="Exhausted Ingestor", version="0.1.0", lifespan=lifespan)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/api/v1/upload", status_code=202)
async def upload_violation(
    video_payload: UploadFile = File(...),
    event_id: str = Form(...),
    recorded_at: str = Form(...),
    peak_decibel: float = Form(...),
    frequency_hz: float = Form(...),
    latitude: float = Form(...),
    longitude: float = Form(...),
    clip_size_bytes: int = Form(...),
    db: Database = Depends(get_db),
):
    if not video_payload.filename or not video_payload.filename.lower().endswith(".mp4"):
        raise HTTPException(400, "Only .mp4 video files are accepted")

    try:
        event_uuid = uuid.UUID(event_id)
    except ValueError:
        raise HTTPException(422, "event_id must be a valid UUID")

    try:
        recorded_dt = datetime.fromisoformat(recorded_at)
    except ValueError:
        raise HTTPException(422, "recorded_at must be a valid ISO 8601 timestamp")

    video_filename = f"{event_id}.mp4"
    video_dir = VIDEO_DIR
    os.makedirs(video_dir, exist_ok=True)
    video_path = os.path.join(video_dir, video_filename)

    content = await video_payload.read()
    async with aiofiles.open(video_path, "wb") as f:
        await f.write(content)

    await db.insert_violation(
        event_id=event_uuid,
        recorded_at=recorded_dt,
        peak_decibel=peak_decibel,
        frequency_hz=frequency_hz,
        latitude=latitude,
        longitude=longitude,
        video_path=video_path,
        clip_size_bytes=clip_size_bytes,
    )

    return {"status": "accepted", "event_id": event_id}

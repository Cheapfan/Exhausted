import os
import tempfile
import uuid
from datetime import datetime, timezone

import pytest
from fastapi.testclient import TestClient

os.environ["DATABASE_URL"] = "postgresql://test:test@localhost:99999/test"
os.environ["VIDEO_DIR"] = tempfile.mkdtemp(prefix="exhausted-test-")

from ingestor.main import app, get_db


class MockDB:
    def __init__(self):
        self.inserted = []

    async def insert_violation(self, **kwargs):
        self.inserted.append(kwargs)


@pytest.fixture
def client():
    mock_db = MockDB()
    app.dependency_overrides[get_db] = lambda: mock_db
    with TestClient(app) as c:
        yield c
    app.dependency_overrides.clear()


def valid_data():
    return {
        "event_id": str(uuid.uuid4()),
        "recorded_at": datetime.now(timezone.utc).isoformat(),
        "peak_decibel": "92.5",
        "frequency_hz": "150.0",
        "latitude": "-33.8688",
        "longitude": "151.2093",
        "clip_size_bytes": "12345",
    }


class TestUpload:
    def test_202_on_valid_upload(self, client):
        data = valid_data()
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 202
        assert resp.json()["status"] == "accepted"

    def test_422_when_event_id_missing(self, client):
        data = valid_data()
        del data["event_id"]
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 422

    def test_422_when_peak_decibel_missing(self, client):
        data = valid_data()
        del data["peak_decibel"]
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 422

    def test_422_on_invalid_uuid(self, client):
        data = valid_data()
        data["event_id"] = "not-a-uuid"
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 422

    def test_422_on_invalid_timestamp(self, client):
        data = valid_data()
        data["recorded_at"] = "not-a-timestamp"
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 422

    def test_400_on_non_mp4_file(self, client):
        data = valid_data()
        files = {"video_payload": ("test.avi", b"fake-video", "video/x-msvideo")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 400
        assert "mp4" in resp.json()["detail"].lower()

    def test_400_on_missing_extension(self, client):
        data = valid_data()
        files = {"video_payload": ("test", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 400
        assert "mp4" in resp.json()["detail"].lower()

    def test_422_on_invalid_int(self, client):
        data = valid_data()
        data["clip_size_bytes"] = "not-a-number"
        files = {"video_payload": ("test.mp4", b"fake-video", "video/mp4")}
        resp = client.post("/api/v1/upload", data=data, files=files)
        assert resp.status_code == 422

    def test_health(self, client):
        resp = client.get("/health")
        assert resp.status_code == 200
        assert resp.json()["status"] == "ok"

## What to build

Connect the edge and cloud by having the GoGopher SyncWorker upload to the real FastAPI Ingestor running in Docker Compose.

- Spin up the cloud Ingestor + PostgreSQL via `docker compose up`
- Run GoGopher locally (on dev machine) pointed at the cloud URL
- Insert a synthetic outbox record (status=0, file_path pointing to a test .mp4)
- Verify the SyncWorker uploads it and a row appears in PostgreSQL
- Verify the file on disk is deleted after successful upload

**Also:** error handling for when the cloud is down
- Start GoGopher without the cloud running
- Verify SyncWorker retries with backoff, does not crash
- Start the cloud — verify the pending record eventually uploads successfully

## Acceptance criteria

- [ ] GoGopher uploads a test .mp4 to real cloud Ingestor
- [ ] Row appears in PostgreSQL with correct metadata
- [ ] Outbox record is deleted after successful upload
- [ ] File on disk is deleted after successful upload
- [ ] Cloud down → SyncWorker retries with backoff, no crash
- [ ] Cloud comes back up → pending record auto-uploads

## Blocked by

- #005 — PanicClipper (for the test .mp4 file format)
- #006 — GoGopher SyncWorker upload logic

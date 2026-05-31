## What to build

A fully automated end-to-end integration test that runs the entire pipeline with mocked hardware on a developer machine, from audio WAV input to processed PostgreSQL record.

**Test script** (`test/integration_test.sh` or `Taskfile.yml`):
1. Generate synthetic test data:
   - WAV file with a 150 Hz sine wave at 94 dB (modified exhaust signature)
   - 150 JPEG frames (static test image or generated pattern)
2. Start Cloud (Docker Compose) — Ingestor + PostgreSQL
3. Start GoGopher with SQLite outbox
4. Start PanicClipper with mock JPEG source
5. Run NoiseComplaint with test WAV → triggers GoGopher
6. Wait for SyncWorker to pick up record → call PanicClipper → encode → upload → to Ingestor
7. Wait for Cloud Processor to pick up clip → infer → blur → store
8. Query PostgreSQL for the processed record
9. Assert: record exists, vehicle_count > 0, has_plate field populated, blur data present, processing_time_ms set
10. Clean up: stop all processes, remove temp files

**Also:** a negative test — feed a WAV with white noise (no trigger band), assert no records were created.

**GitHub Actions / local CI:** `make test-integration` target that runs the full pipeline.

## Acceptance criteria

- [ ] Full pipeline test passes: WAV → SQLite → .mp4 → upload → PostgreSQL → processed
- [ ] Negative test passes: white noise WAV → no records
- [ ] Integration test is fully automated (one command)
- [ ] Cleanup removes all temp files and Docker containers
- [ ] Test runs on a laptop without any special hardware

## Blocked by

- #003 — NoiseComplaint trigger loop
- #007 — Edge-to-cloud upload integration
- #008 — Cloud Processor

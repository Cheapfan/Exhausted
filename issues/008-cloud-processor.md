## What to build

The cloud processing backend: ONNX Runtime inference pipeline, vectorized Gaussian blur, and a background worker that polls PostgreSQL for unprocessed clips.

**Inference engine** (`cloud/processor/inference.py`):
- `SilencerInferenceEngine` class:
  - Loads INT8 quantized YOLOv8m ONNX model
  - ONNX Runtime session with `CPUExecutionProvider`, `intra_op_num_threads = cpu_count()`, full graph optimization
  - `preprocess(frame: np.ndarray) -> np.ndarray` — resize to 640×640, HWC→CHW, normalize [0,1]
  - `infer(video_path: str) -> list[dict]` — opens video, processes each frame, extracts detections
  - `parse_outputs(outputs, original_shape) -> list[dict]` — applies NMS, rescales to original image dimensions, returns `[{class, confidence, x1, y1, x2, y2}]`
  - Filter: keep only vehicle (car, motorcycle, bus, truck), license plate, and person classes

**Anonymizer** (`cloud/processor/anonymizer.py`):
- `apply_vectorized_blur(frame: np.ndarray, bounding_boxes: list[dict]) -> np.ndarray`
- For each box: clamp to frame bounds, extract ROI, compute kernel size as 25% of box dimension (forced odd), apply `cv2.GaussianBlur`, blit back

**Background processor** (`cloud/processor/worker.py`):
- Polls `violations` table for `processing_status = 0` every 5 seconds
- For each: read video → run inference → collect all frames' detections → apply blur per frame → update record with JSONB results and `processing_status = 1`
- Records per-violation: vehicle count, vehicle list, plate bounding boxes, face bounding boxes, blur kernel sizes, processing time

**ONNX model:**
- Download script or instructions to obtain `yolov8m_int8.onnx` (from ONNX Model Zoo or export from Ultralytics)

**Tests:**
- Inference with a known test image → assert output shape and class values
- Anonymizer with synthetic frame + known box → assert pixel values inside box are blurred
- Processor end-to-end: drop a clip into the queue → poll → verify processed result

## Acceptance criteria

- [ ] `SilencerInferenceEngine` loads a YOLOv8 ONNX model successfully
- [ ] Single frame inference returns expected bounding box structure
- [ ] Full video inference processes all frames without error
- [ ] Anonymizer blurs the correct region without modifying outside pixels
- [ ] Kernel sizes are always odd integers
- [ ] Processor picks up unprocessed records from DB
- [ ] After processing, record has `processing_status = 1`, populated JSONB fields, `processing_time_ms`
- [ ] `vehicle_count` matches the number of vehicle detections

## Blocked by

- #002 — Cloud Ingestor with PostgreSQL schema

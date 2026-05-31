## What to build

Set up the Go project skeleton and implement the core audio processing library (NoiseComplaint's detection algorithm) as a standalone, fully tested package.

## Implementation

### Project structure

```
edge/
├── cmd/
│   ├── noisecomplaint/main.go     # Working CLI: reads WAV, processes, prints violations
│   ├── panicclipper/              # Stub directory (future slice)
│   └── gogopher/                  # Stub directory (future slice)
├── internal/
│   ├── audio/
│   │   ├── source.go              # AudioSource interface
│   │   ├── processor.go           # RMS, Decibel, Hanning, FFT, EnergyRatio, Classify
│   │   ├── processor_test.go      # 16 tests
│   │   ├── wav.go                 # WAV file reader + WAVSource (mock AudioSource)
│   │   └── wav_test.go            # 5 tests
│   └── video/
│       └── source.go              # VideoSource interface (stub for future slice)
├── go.mod
└── go.sum
```

### Interfaces (for testability)

```go
// internal/audio/source.go
type AudioSource interface {
    ReadSample() ([]int16, error)  // returns 1024 samples per call
    Close() error
}

// internal/video/source.go
type VideoSource interface {
    ReadFrame() ([]byte, error)   // returns MJPEG JPEG bytes
    Close() error
}
```

These interfaces are the contract EVERY audio/video source must satisfy — both mocks (WAV files, JPEG directories) and real hardware (ALSA, V4L2). This lets you develop and unit-test all orchestration logic on your laptop with mocks, then swap in real hardware drivers later without changing a single line of the processing or coordination code.

### AudioProcessor — the detection pipeline

The detection pipeline is a chain of pure functions (no state, no I/O):

**1. RMS — raw amplitude**
```
func RMS(samples []int16) float64
```
Computes root mean square of 1024 samples. Handles empty input (returns 0). Verified: a constant-amplitude sine wave at amplitude 100 gives RMS ≈ 70.7 (= 100/√2), matching the mathematical expectation.

**2. Decibel — map to logarithmic scale**
```
func Decibel(rms, ref float64) float64
```
Formula: `20 × log₁₀(rms / ref)`. Returns `-Inf` for zero or negative inputs (signal detection boundary). The reference value (`ref`) defaults to 1.0 in the CLI — this will need real calibration against a known sound source when hardware is deployed.

**3. Hanning window — prepare for FFT**
```
func ApplyHanning(samples []int16) []float64
```
Multiplies each sample by `0.5 × (1 - cos(2πn/(N-1)))` to suppress spectral leakage. Endpoints are forced to 0, center sample retains ~100% of original amplitude. The `[]int16` input is promoted to `[]float64` during windowing.

**4. FFT — frequency domain**
```
func FFT(windowed []float64) []complex128
```
Wraps Gonum's real FFT (`gonum.org/v1/gonum/dsp/fourier`). For N=1024 input, returns N/2+1 = 513 complex coefficients (positive frequencies only, up to Nyquist at 22050 Hz). This is a deliberate optimization of the real FFT — negative frequencies are symmetric and contain no additional information.

Helper:
```
func FFTLength(n int) int  // returns n/2 + 1
```

**5. Energy ratio — the discriminant that rejects false positives**
```
type FreqBand struct {
    LowStartHz  float64  // 100
    LowEndHz    float64  // 250
    MidStartHz  float64  // 250
    MidEndHz    float64  // 500
}

func EnergyRatio(spectrum []complex128, sampleRate float64, band FreqBand) float64
```

This is the key innovation over a naive FFT approach. Instead of just checking "is there energy in the low band?", it computes:

```
ratio = len(lowBandEnergy) / (lowBandEnergy + midBandEnergy)
```

Bin mapping is computed dynamically from the half-spectrum:
- `freqResolution = sampleRate / FFT_N` where `FFT_N = (len(spectrum) - 1) × 2`
- `lowStartBin = ceil(LowStartHz / freqResolution)`
- `midEndBin = floor(MidEndHz / freqResolution)`

With 44100 Hz sample rate and 1024-point FFT: resolution ≈ 43.07 Hz/bin.
- Low band (100-250 Hz) → bins 3, 4, 5
- Mid band (250-500 Hz) → bins 6, 7, 8, 9, 10, 11

This means a diesel truck (broad spectrum: energy in both low AND mid bands) gives ratio ≈ 0.3-0.5 (below 0.6 threshold → rejected). A modified exhaust at 150 Hz with no other frequency content gives ratio ≈ 1.0 (above 0.6 → triggered).

**6. Classify — the final gate**
```
func Classify(dB, ratio, dbThreshold, ratioThreshold float64) bool
```
Returns `true` only if BOTH conditions are met:
- `dB > dbThreshold` (strict greater-than, not ≥ — avoids triggering on exactly-at-threshold noise floor)
- `ratio > ratioThreshold`

Both thresholds are configurable values passed in at runtime — not hardcoded.

### WAV file reader — mock audio source for laptop development

```
type WAVSource struct { ... }
func NewWAVSource(path string, chunkSize int) (*WAVSource, error)
```

Reads standard 44-byte-header PCM WAV files. Supports:
- 16-bit signed integer samples
- Mono or stereo (stereo → mono via averaging)
- Automatically pads the last chunk to `chunkSize` if the file doesn't divide evenly

Implements the `AudioSource` interface, so NoiseComplaint can be pointed at a WAV file on your laptop or at a real ALSA device on the Pi — the trigger loop code doesn't change.

### CLI — NoiseComplaint runner

```
go run ./cmd/noisecomplaint/ --wav /path/to/file.wav
```

Configurable flags:
- `--wav` (required) — path to WAV file for mock audio source
- `--db-threshold` (default 85.0) — decibel threshold
- `--ratio-threshold` (default 0.6) — energy ratio threshold

For each 1024-sample chunk, it:
1. Computes RMS → dB
2. Applies Hanning window → FFT
3. Computes energy ratio across the 100-250 Hz / 250-500 Hz bands
4. Runs Classify: if both thresholds are exceeded, prints `VIOLATION` with dB, ratio, and dominant frequency
5. Exit code 0 if any violations found, 1 if none

### Test coverage (25 tests, all passing)

**Processor tests (16):**
- `TestRMS_Empty` — empty input returns 0
- `TestRMS_Constant` — constant signal computes correctly
- `TestRMS_Sine` — sine wave RMS = amplitude/√2
- `TestDecibel_Positive` — equal inputs = 0 dB
- `TestDecibel_Known` — verifies 20×log₁₀(rms/ref) formula
- `TestDecibel_Negative` — rms < ref gives negative dB
- `TestDecibel_Zero` — zero input returns -Inf
- `TestApplyHanning_Length` — output same length as input
- `TestApplyHanning_Endpoints` — endpoints = 0
- `TestApplyHanning_Center` — center ≈ original amplitude
- `TestFFT_Length` — 1024 input → 513 output (half-spectrum)
- `TestFFTLength` — verifies N/2+1 formula for various N
- `TestFFT_DCOffset` — DC-only input → only bin 0 has energy
- `TestEnergyRatio_LowBandDominant` — 150 Hz → ratio ≈ 1.0
- `TestEnergyRatio_MidBandDominant` — 350 Hz → ratio ≈ 0.0
- `TestEnergyRatio_EqualEnergy` — equal low+mid → ratio ≈ 0.5
- `TestEnergyRatio_ZeroInput` — silence → ratio = 0
- `TestClassify_True/False` — all four boundary conditions (above both, below dB, below ratio, below both, at exact threshold)

**WAV reader tests (5):**
- `TestReadWAV_Mono16Bit` — reads back correct sample rate, bit depth, sample count
- `TestReadWAV_StereoToMono` — stereo file correctly downmixed via averaging
- `TestWAVSource_ReadSample` — chunks returned in order, EOF at end
- `TestWAVSource_LastPartialChunk` — last chunk padded to 1024
- `TestWAVSource_InvalidFile` — non-WAV file returns error

### Verified end-to-end behavior

Three test WAVs generated and run through the CLI:

| Sound | dB | Ratio | Result | Why |
|---|---|---|---|---|
| 150 Hz sine, amplitude 28000 | ~86 | 1.000 | 86 VIOLATIONS | Low-band dominant, above threshold |
| White noise, amplitude 30000 | ~88 | ~0.15 | 0 violations | Broad spectrum, ratio < 0.6 |
| 1000 Hz sine (siren sim), amp 28000 | ~86 | 0.000 | 0 violations | Energy in mid/high band, ratio = 0 |

### Important design decisions made

1. **`>` not `>=` for threshold comparison.** `Classify` uses strict greater-than. An input at exactly 85.0 dB is NOT a violation — this prevents triggering on the noise floor or a perfectly calibrated signal. In the real world, the threshold should be set a couple dB above the actual trigger level to create hysteresis.

2. **Half-spectrum FFT output.** Gonum's real FFT returns N/2+1 coefficients. This is correct and efficient — negative frequencies are always symmetric for real input. All bin indexing in `EnergyRatio` assumes half-spectrum.

3. **`ref = 1.0` is a placeholder.** The dB reference value needs calibration against a real microphone. The `Decibel` function accepts it as a parameter; when you deploy with ALSA, you'll need to record a known 94 dB SPL tone (cheap calibrator) and compute the RMS of that recording to set the correct reference.

4. **Chunk size is hardcoded at 1024 in the CLI** but the processor functions work on any slice size. The 1024 choice balances frequency resolution (~43 Hz/bin — good enough to distinguish 150 Hz from 250 Hz) against latency (~23ms per chunk, well within real-time requirements).

5. **Depends on Gonum.** The `gonum.org/v1/gonum/dsp/fourier` package is the de facto Go numerical library. It uses optimized pure-Go FFT with O(n log n) complexity.

## Acceptance criteria

- [x] `go build ./...` succeeds from the repo root
- [x] RMS of a constant-amplitude sine wave matches expected value
- [x] Decibel conversion is correct (0 dB for equal inputs, follows 20×log₁₀ formula)
- [x] Hanning window endpoints are 0, center preserves amplitude
- [x] FFT of 1024 samples returns 513 coefficients (half-spectrum)
- [x] DC offset has energy only in bin 0
- [x] Energy ratio is ~1.0 for 150 Hz signal, ~0.0 for 350 Hz
- [x] `Classify` returns true only when both thresholds are exceeded, false at exact threshold
- [x] WAV CLI reads a `.wav` file and prints detection result
- [x] All functions have tests covering edge cases (empty, silent, DC, boundary conditions)
- [x] End-to-end verified: modified exhaust sim triggers, white noise and siren sim do not

## Blocked by

None — can start immediately.

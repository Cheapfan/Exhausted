package audio

import (
	"math"
	"testing"
)

func TestRMS_Empty(t *testing.T) {
	if got := RMS(nil); got != 0 {
		t.Errorf("RMS(nil) = %f; want 0", got)
	}
	if got := RMS([]int16{}); got != 0 {
		t.Errorf("RMS([]) = %f; want 0", got)
	}
}

func TestRMS_Constant(t *testing.T) {
	samples := make([]int16, 100)
	for i := range samples {
		samples[i] = 100
	}
	got := RMS(samples)
	want := 100.0
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("RMS(constant 100) = %f; want %f", got, want)
	}
}

func TestRMS_Sine(t *testing.T) {
	n := 1024
	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		samples[i] = int16(100.0 * math.Sin(2*math.Pi*150*float64(i)/44100))
	}
	got := RMS(samples)
	expected := 100.0 / math.Sqrt2
	if math.Abs(got-expected) > 0.5 {
		t.Errorf("RMS(sine) = %f; want ~%f", got, expected)
	}
}

func TestDecibel_Positive(t *testing.T) {
	got := Decibel(1.0, 1.0)
	if got != 0 {
		t.Errorf("Decibel(1,1) = %f; want 0", got)
	}
}

func TestDecibel_Known(t *testing.T) {
	got := Decibel(2.0, 1.0)
	want := 20 * math.Log10(2)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("Decibel(2,1) = %f; want %f", got, want)
	}
}

func TestDecibel_Negative(t *testing.T) {
	got := Decibel(1.0, 2.0)
	want := 20 * math.Log10(0.5)
	if math.Abs(got-want) > 1e-10 {
		t.Errorf("Decibel(1,2) = %f; want %f", got, want)
	}
}

func TestDecibel_Zero(t *testing.T) {
	got := Decibel(0, 1)
	if got != -math.MaxFloat64 {
		t.Errorf("Decibel(0,1) = %f; want -Inf (%f)", got, -math.MaxFloat64)
	}
}

func TestApplyHanning_Length(t *testing.T) {
	samples := []int16{1, 2, 3, 4}
	got := ApplyHanning(samples)
	if len(got) != 4 {
		t.Errorf("len = %d; want 4", len(got))
	}
}

func TestApplyHanning_Endpoints(t *testing.T) {
	samples := []int16{100, 100, 100, 100}
	got := ApplyHanning(samples)
	if got[0] != 0 {
		t.Errorf("hanning[0] = %f; want 0 (window at endpoint)", got[0])
	}
	if got[len(got)-1] != 0 {
		t.Errorf("hanning[last] = %f; want 0 (window at endpoint)", got[len(got)-1])
	}
}

func TestApplyHanning_Center(t *testing.T) {
	n := 101
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = 100
	}
	got := ApplyHanning(samples)
	mid := n / 2
	if got[mid] <= 95 || got[mid] > 101 {
		t.Errorf("hanning[center] = %f; want ~100 (center of window)", got[mid])
	}
}

func TestFFT_Length(t *testing.T) {
	input := make([]float64, 1024)
	for i := range input {
		input[i] = 1
	}
	spectrum := FFT(input)
	want := 1024/2 + 1
	if len(spectrum) != want {
		t.Errorf("FFT output length = %d; want %d", len(spectrum), want)
	}
}

func TestFFTLength(t *testing.T) {
	cases := []struct{ n, want int }{
		{1024, 513},
		{512, 257},
		{256, 129},
	}
	for _, c := range cases {
		if got := FFTLength(c.n); got != c.want {
			t.Errorf("FFTLength(%d) = %d; want %d", c.n, got, c.want)
		}
	}
}

func TestFFT_DCOffset(t *testing.T) {
	n := 1024
	input := make([]float64, n)
	for i := range input {
		input[i] = 100
	}
	spectrum := FFT(input)
	if real(spectrum[0]) <= 0 {
		t.Errorf("DC component = %f; want > 0 for DC offset", real(spectrum[0]))
	}
	for k := 1; k < len(spectrum); k++ {
		if real(spectrum[k]) != 0 || imag(spectrum[k]) != 0 {
			t.Errorf("Non-DC bin %d has non-zero coefficient for DC-only input", k)
		}
	}
}

func newHalfSpectrum(fftSize int) []complex128 {
	return make([]complex128, fftSize/2+1)
}

func TestEnergyRatio_LowBandDominant(t *testing.T) {
	sampleRate := 44100.0
	fftSize := 1024
	freqRes := sampleRate / float64(fftSize)

	lowBin := int(150 / freqRes)

	spectrum := newHalfSpectrum(fftSize)
	spectrum[lowBin] = complex(100, 0)
	spectrum[lowBin+1] = complex(10, 0)

	band := FreqBand{
		LowStartHz: 100, LowEndHz: 250,
		MidStartHz: 250, MidEndHz: 500,
	}
	ratio := EnergyRatio(spectrum, sampleRate, band)
	if ratio < 0.95 {
		t.Errorf("EnergyRatio(low dominant) = %f; want > 0.95", ratio)
	}
}

func TestEnergyRatio_MidBandDominant(t *testing.T) {
	sampleRate := 44100.0
	fftSize := 1024
	freqRes := sampleRate / float64(fftSize)

	midBin := int(350 / freqRes)

	spectrum := newHalfSpectrum(fftSize)
	spectrum[midBin] = complex(100, 0)

	band := FreqBand{
		LowStartHz: 100, LowEndHz: 250,
		MidStartHz: 250, MidEndHz: 500,
	}
	ratio := EnergyRatio(spectrum, sampleRate, band)
	if ratio > 0.05 {
		t.Errorf("EnergyRatio(mid dominant) = %f; want < 0.05", ratio)
	}
}

func TestEnergyRatio_EqualEnergy(t *testing.T) {
	sampleRate := 44100.0
	fftSize := 1024
	freqRes := sampleRate / float64(fftSize)

	lowBin := int(150 / freqRes)
	midBin := int(350 / freqRes)

	spectrum := newHalfSpectrum(fftSize)
	spectrum[lowBin] = complex(100, 0)
	spectrum[midBin] = complex(100, 0)

	band := FreqBand{
		LowStartHz: 100, LowEndHz: 250,
		MidStartHz: 250, MidEndHz: 500,
	}
	ratio := EnergyRatio(spectrum, sampleRate, band)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("EnergyRatio(equal energy) = %f; want ~0.5", ratio)
	}
}

func TestEnergyRatio_ZeroInput(t *testing.T) {
	spectrum := newHalfSpectrum(1024)
	band := FreqBand{
		LowStartHz: 100, LowEndHz: 250,
		MidStartHz: 250, MidEndHz: 500,
	}
	ratio := EnergyRatio(spectrum, 44100, band)
	if ratio != 0 {
		t.Errorf("EnergyRatio(zero) = %f; want 0", ratio)
	}
}

func TestClassify_True(t *testing.T) {
	if !Classify(90, 0.7, 85, 0.6) {
		t.Error("Classify(90, 0.7, 85, 0.6) = false; want true")
	}
}

func TestClassify_BelowDB(t *testing.T) {
	if Classify(80, 0.7, 85, 0.6) {
		t.Error("Classify(80, 0.7, 85, 0.6) = true; want false")
	}
}

func TestClassify_BelowRatio(t *testing.T) {
	if Classify(90, 0.5, 85, 0.6) {
		t.Error("Classify(90, 0.5, 85, 0.6) = true; want false")
	}
}

func TestClassify_BelowBoth(t *testing.T) {
	if Classify(80, 0.5, 85, 0.6) {
		t.Error("Classify(80, 0.5, 85, 0.6) = true; want false")
	}
}

func TestClassify_AtThreshold(t *testing.T) {
	if Classify(85, 0.6, 85, 0.6) {
		t.Error("Classify(85, 0.6, 85, 0.6) = true at exact threshold; want false (> not >=)")
	}
}

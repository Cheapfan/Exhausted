package audio

import (
	"math"

	"gonum.org/v1/gonum/dsp/fourier"
)

func RMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		f := float64(s)
		sum += f * f
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func Decibel(rms, ref float64) float64 {
	if rms <= 0 || ref <= 0 {
		return -math.MaxFloat64
	}
	return 20 * math.Log10(rms/ref)
}

func ApplyHanning(samples []int16) []float64 {
	n := len(samples)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		window := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
		out[i] = float64(samples[i]) * window
	}
	return out
}

func FFT(windowed []float64) []complex128 {
	n := len(windowed)
	fft := fourier.NewFFT(n)
	return fft.Coefficients(nil, windowed)
}

func FFTLength(n int) int {
	return n/2 + 1
}

type FreqBand struct {
	LowStartHz  float64
	LowEndHz    float64
	MidStartHz  float64
	MidEndHz    float64
}

func EnergyRatio(spectrum []complex128, sampleRate float64, band FreqBand) float64 {
	n := (len(spectrum) - 1) * 2
	freqRes := sampleRate / float64(n)

	lowStartBin := int(math.Ceil(band.LowStartHz / freqRes))
	lowEndBin := int(math.Floor(band.LowEndHz / freqRes))
	midStartBin := int(math.Ceil(band.MidStartHz / freqRes))
	midEndBin := int(math.Floor(band.MidEndHz / freqRes))

	if lowEndBin >= n {
		lowEndBin = n - 1
	}
	if midEndBin >= n {
		midEndBin = n - 1
	}

	var lowEnergy, midEnergy float64
	for k := lowStartBin; k <= lowEndBin && k < n; k++ {
		lowEnergy += real(spectrum[k]) * real(spectrum[k]) + imag(spectrum[k]) * imag(spectrum[k])
	}
	for k := midStartBin; k <= midEndBin && k < n; k++ {
		midEnergy += real(spectrum[k]) * real(spectrum[k]) + imag(spectrum[k]) * imag(spectrum[k])
	}

	total := lowEnergy + midEnergy
	if total == 0 {
		return 0
	}
	return lowEnergy / total
}

func Classify(dB, ratio, dbThreshold, ratioThreshold float64) bool {
	return dB > dbThreshold && ratio > ratioThreshold
}

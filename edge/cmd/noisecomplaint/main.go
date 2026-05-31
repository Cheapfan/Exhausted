package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gonum.org/v1/gonum/dsp/fourier"

	"github.com/steven-cokro/exhausted/internal/audio"
)

func main() {
	wavPath := flag.String("wav", "", "path to WAV file for mock audio source")
	dbThreshold := flag.Float64("db-threshold", 85.0, "decibel threshold for triggering")
	ratioThreshold := flag.Float64("ratio-threshold", 0.6, "energy ratio threshold for triggering")
	flag.Parse()

	if *wavPath == "" {
		log.Fatal("--wav is required")
	}

	sampleRate := 44100
	chunkSize := 1024

	source, err := audio.NewWAVSource(*wavPath, chunkSize)
	if err != nil {
		log.Fatalf("failed to open WAV: %v", err)
	}
	defer source.Close()

	band := audio.FreqBand{
		LowStartHz: 100, LowEndHz: 250,
		MidStartHz: 250, MidEndHz: 500,
	}

	fft := fourier.NewFFT(chunkSize)

	fmt.Printf("Processing %s...\n", *wavPath)
	fmt.Printf("Thresholds: dB > %.1f, ratio > %.2f\n", *dbThreshold, *ratioThreshold)
	fmt.Println()

	chunkNum := 0
	triggerCount := 0
	for {
		samples, err := source.ReadSample()
		if err != nil {
			break
		}

		rms := audio.RMS(samples)
		db := audio.Decibel(rms, 1.0)
		windowed := audio.ApplyHanning(samples)
		spectrum := fft.Coefficients(nil, windowed)
		ratio := audio.EnergyRatio(spectrum, float64(sampleRate), band)
		triggered := audio.Classify(db, ratio, *dbThreshold, *ratioThreshold)

		if triggered {
			triggerCount++
			dominantFreq := findDominantFrequency(spectrum, float64(sampleRate))
			fmt.Printf("VIOLATION #%d | chunk=%4d | dB=%6.1f | ratio=%.3f | dominant=%.1f Hz\n",
				triggerCount, chunkNum, db, ratio, dominantFreq)
		}

		chunkNum++
	}

	fmt.Printf("\nDone. Processed %d chunks, %d violations detected.\n", chunkNum, triggerCount)

	if triggerCount > 0 {
		os.Exit(0)
	}
	os.Exit(1)
}

func findDominantFrequency(spectrum []complex128, sampleRate float64) float64 {
	if len(spectrum) == 0 {
		return 0
	}

	fftSize := (len(spectrum) - 1) * 2

	peakMag := 0.0
	peakBin := 0
	for k := 1; k < len(spectrum); k++ {
		mag := real(spectrum[k])*real(spectrum[k]) + imag(spectrum[k])*imag(spectrum[k])
		if mag > peakMag {
			peakMag = mag
			peakBin = k
		}
	}

	return float64(peakBin) * sampleRate / float64(fftSize)
}

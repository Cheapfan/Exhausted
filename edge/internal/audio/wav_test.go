package audio

import (
	"io"
	"math"
	"os"
	"testing"
)

func generateTestWAV(t *testing.T, path string, sampleRate int, samples []int16) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	dataSize := len(samples) * 2
	header := make([]byte, 44)

	copy(header[0:4], "RIFF")
	binaryLittleEndianPutUint32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binaryLittleEndianPutUint32(header[16:20], 16)
	binaryLittleEndianPutUint16(header[20:22], 1)
	binaryLittleEndianPutUint16(header[22:24], 1)
	binaryLittleEndianPutUint32(header[24:28], uint32(sampleRate))
	binaryLittleEndianPutUint32(header[28:32], uint32(sampleRate*2))
	binaryLittleEndianPutUint16(header[32:34], 2)
	binaryLittleEndianPutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binaryLittleEndianPutUint32(header[40:44], uint32(dataSize))

	if _, err := f.Write(header); err != nil {
		t.Fatal(err)
	}
	for _, s := range samples {
		if err := binaryLittleEndianPutInt16(f, s); err != nil {
			t.Fatal(err)
		}
	}
}

func binaryLittleEndianPutUint32(p []byte, v uint32) {
	p[0] = byte(v)
	p[1] = byte(v >> 8)
	p[2] = byte(v >> 16)
	p[3] = byte(v >> 24)
}

func binaryLittleEndianPutUint16(p []byte, v uint16) {
	p[0] = byte(v)
	p[1] = byte(v >> 8)
}

func binaryLittleEndianPutInt16(f *os.File, v int16) error {
	_, err := f.Write([]byte{byte(v), byte(v >> 8)})
	return err
}

func TestReadWAV_Mono16Bit(t *testing.T) {
	path := t.TempDir() + "/test_mono.wav"
	sampleRate := 44100
	n := 1024
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(100 * math.Sin(2*math.Pi*150*float64(i)/float64(sampleRate)))
	}
	generateTestWAV(t, path, sampleRate, samples)

	wav, err := readWAV(mustOpen(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if wav.SampleRate != sampleRate {
		t.Errorf("SampleRate = %d; want %d", wav.SampleRate, sampleRate)
	}
	if wav.BitsPerSample != 16 {
		t.Errorf("BitsPerSample = %d; want 16", wav.BitsPerSample)
	}
	if len(wav.Data) != n {
		t.Errorf("len(Data) = %d; want %d", len(wav.Data), n)
	}
}

func TestReadWAV_StereoToMono(t *testing.T) {
	path := t.TempDir() + "/test_stereo.wav"
	sampleRate := 44100
	n := 1024

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	dataSize := n * 2 * 2
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binaryLittleEndianPutUint32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binaryLittleEndianPutUint32(header[16:20], 16)
	binaryLittleEndianPutUint16(header[20:22], 1)
	binaryLittleEndianPutUint16(header[22:24], 2)
	binaryLittleEndianPutUint32(header[24:28], uint32(sampleRate))
	binaryLittleEndianPutUint32(header[28:32], uint32(sampleRate*4))
	binaryLittleEndianPutUint16(header[32:34], 4)
	binaryLittleEndianPutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binaryLittleEndianPutUint32(header[40:44], uint32(dataSize))
	f.Write(header)

	for i := 0; i < n; i++ {
		val := int16(100 * math.Sin(2*math.Pi*150*float64(i)/float64(sampleRate)))
		binaryLittleEndianPutInt16(f, val)
		binaryLittleEndianPutInt16(f, val)
	}
	f.Close()

	wav, err := readWAV(mustOpen(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if wav.NumChannels != 2 {
		t.Errorf("NumChannels = %d; want 2", wav.NumChannels)
	}
	if len(wav.Data) != n {
		t.Errorf("len(Data) after stereo->mono = %d; want %d", len(wav.Data), n)
	}
}

func TestWAVSource_ReadSample(t *testing.T) {
	path := t.TempDir() + "/test_source.wav"
	n := 2048
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(i)
	}
	generateTestWAV(t, path, 44100, samples)

	src, err := NewWAVSource(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	chunk1, err := src.ReadSample()
	if err != nil {
		t.Fatal(err)
	}
	if len(chunk1) != 1024 {
		t.Errorf("first chunk len = %d; want 1024", len(chunk1))
	}
	if chunk1[0] != 0 {
		t.Errorf("first sample = %d; want 0", chunk1[0])
	}

	chunk2, err := src.ReadSample()
	if err != nil {
		t.Fatal(err)
	}
	if len(chunk2) != 1024 {
		t.Errorf("second chunk len = %d; want 1024", len(chunk2))
	}

	_, err = src.ReadSample()
	if err != io.EOF {
		t.Errorf("third read err = %v; want EOF", err)
	}
}

func TestWAVSource_LastPartialChunk(t *testing.T) {
	path := t.TempDir() + "/test_partial.wav"
	n := 1100
	samples := make([]int16, n)
	generateTestWAV(t, path, 44100, samples)

	src, err := NewWAVSource(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	chunk1, _ := src.ReadSample()
	if len(chunk1) != 1024 {
		t.Errorf("first chunk = %d; want 1024", len(chunk1))
	}

	chunk2, err := src.ReadSample()
	if err != nil {
		t.Fatal(err)
	}
	if len(chunk2) != 1024 {
		t.Errorf("last chunk padded = %d; want 1024 (padded)", len(chunk2))
	}

	_, err = src.ReadSample()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestWAVSource_InvalidFile(t *testing.T) {
	path := t.TempDir() + "/not_wav"
	os.WriteFile(path, []byte("not a wav"), 0644)

	_, err := NewWAVSource(path, 1024)
	if err == nil {
		t.Error("expected error for invalid WAV file")
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type WAVFile struct {
	SampleRate int
	NumChannels int
	BitsPerSample int
	Data []int16
}

type WAVSource struct {
	file    *os.File
	buf     []int16
	pos     int
	chunkSize int
}

func NewWAVSource(path string, chunkSize int) (*WAVSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wav: %w", err)
	}

	wav, err := readWAV(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("read wav header: %w", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, fmt.Errorf("seek: %w", err)
	}

	return &WAVSource{
		file:      f,
		buf:       wav.Data,
		chunkSize: chunkSize,
	}, nil
}

func (w *WAVSource) ReadSample() ([]int16, error) {
	if w.pos >= len(w.buf) {
		return nil, io.EOF
	}

	end := w.pos + w.chunkSize
	if end > len(w.buf) {
		end = len(w.buf)
	}

	chunk := w.buf[w.pos:end]
	w.pos = end

	if len(chunk) < w.chunkSize {
		padded := make([]int16, w.chunkSize)
		copy(padded, chunk)
		return padded, nil
	}

	return chunk, nil
}

func (w *WAVSource) Close() error {
	return w.file.Close()
}

func readWAV(r io.ReadSeeker) (*WAVFile, error) {
	header := make([]byte, 44)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file")
	}

	audioFormat := binary.LittleEndian.Uint16(header[20:22])
	if audioFormat != 1 {
		return nil, fmt.Errorf("unsupported audio format %d (only PCM supported)", audioFormat)
	}

	numChannels := int(binary.LittleEndian.Uint16(header[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(header[24:28]))
	bitsPerSample := int(binary.LittleEndian.Uint16(header[34:36]))

	if bitsPerSample != 16 {
		return nil, fmt.Errorf("unsupported bit depth %d (only 16-bit supported)", bitsPerSample)
	}

	if _, err := r.Seek(44, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to data: %w", err)
	}

	var samples []int16
	for {
		var sample int16
		if err := binary.Read(r, binary.LittleEndian, &sample); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read sample: %w", err)
		}
		samples = append(samples, sample)
	}

	if numChannels > 1 {
		mono := make([]int16, 0, len(samples)/numChannels)
		for i := 0; i+numChannels <= len(samples); i += numChannels {
			var sum int64
			for ch := 0; ch < numChannels; ch++ {
				sum += int64(samples[i+ch])
			}
			mono = append(mono, int16(sum/int64(numChannels)))
		}
		samples = mono
	}

	return &WAVFile{
		SampleRate:    sampleRate,
		NumChannels:   numChannels,
		BitsPerSample: bitsPerSample,
		Data:          samples,
	}, nil
}

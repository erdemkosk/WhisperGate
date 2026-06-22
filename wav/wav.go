package wav

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// WavHeader represents a standard 16-bit Mono PCM WAV header (44 bytes).
type WavHeader struct {
	ChunkID       [4]byte // "RIFF"
	ChunkSize     uint32  // 36 + Subchunk2Size
	Format        [4]byte // "WAVE"
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16  // 1 for Mono
	SampleRate    uint32  // e.g. 44100
	ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // NumChannels * BitsPerSample/8
	BitsPerSample uint16  // 16
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32  // NumSamples * NumChannels * BitsPerSample/8
}

// WriteWAV writes float64 samples to a 16-bit mono PCM WAV file.
// The samples should be in the range [-1.0, 1.0].
func WriteWAV(filename string, samples []float64, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	numSamples := len(samples)
	subchunk2Size := uint32(numSamples * 2) // 2 bytes per sample (16-bit)
	header := WavHeader{
		ChunkSize:     36 + subchunk2Size,
		Subchunk1Size: 16,
		AudioFormat:   1, // PCM
		NumChannels:   1, // Mono
		SampleRate:    uint32(sampleRate),
		ByteRate:      uint32(sampleRate * 2), // sampleRate * 1 channel * 2 bytes
		BlockAlign:    2,                      // 1 channel * 2 bytes
		BitsPerSample: 16,
		Subchunk2Size: subchunk2Size,
	}

	copy(header.ChunkID[:], "RIFF")
	copy(header.Format[:], "WAVE")
	copy(header.Subchunk1ID[:], "fmt ")
	copy(header.Subchunk2ID[:], "data")

	// Write header
	err = binary.Write(file, binary.LittleEndian, &header)
	if err != nil {
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Write audio data
	for _, sample := range samples {
		// Clamp to [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		// Convert to 16-bit PCM (signed int16)
		val := int16(math.Round(sample * 32767.0))
		err = binary.Write(file, binary.LittleEndian, val)
		if err != nil {
			return fmt.Errorf("failed to write audio sample: %w", err)
		}
	}

	return nil
}

// ReadWAV reads a 16-bit PCM WAV file and returns the float64 samples (in range [-1.0, 1.0]) and the sample rate.
func ReadWAV(filename string) ([]float64, int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var header WavHeader
	err = binary.Read(file, binary.LittleEndian, &header)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Basic validation
	if string(header.ChunkID[:]) != "RIFF" || string(header.Format[:]) != "WAVE" {
		return nil, 0, fmt.Errorf("invalid WAV format")
	}
	if string(header.Subchunk1ID[:]) != "fmt " {
		return nil, 0, fmt.Errorf("invalid format subchunk ID")
	}
	if header.AudioFormat != 1 {
		return nil, 0, fmt.Errorf("unsupported audio format (only uncompressed PCM supported, got %d)", header.AudioFormat)
	}
	if header.BitsPerSample != 16 {
		return nil, 0, fmt.Errorf("unsupported bits per sample (only 16-bit supported, got %d)", header.BitsPerSample)
	}

	// Move file pointer to start of data subchunk if we skipped metadata chunk (like LIST or JUNK)
	// Some WAV encoders put LIST or other chunks before the "data" chunk.
	// Let's handle this robustly by reading headers and finding "data".
	if string(header.Subchunk2ID[:]) != "data" {
		// Seek back to just after fmt chunk and find "data" chunk
		// Standard offset of fmt chunk is 12 (RIFF header) + 8 (fmt ID and size) + Subchunk1Size
		offset := int64(12 + 8 + header.Subchunk1Size)
		_, err = file.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to seek: %w", err)
		}

		var chunkID [4]byte
		var chunkSize uint32
		found := false
		for {
			err = binary.Read(file, binary.LittleEndian, &chunkID)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, 0, fmt.Errorf("failed to read chunk ID: %w", err)
			}
			err = binary.Read(file, binary.LittleEndian, &chunkSize)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to read chunk size: %w", err)
			}

			if string(chunkID[:]) == "data" {
				header.Subchunk2ID = chunkID
				header.Subchunk2Size = chunkSize
				found = true
				break
			}

			// Seek past this chunk
			_, err = file.Seek(int64(chunkSize), io.SeekCurrent)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to seek past chunk: %w", err)
			}
		}

		if !found {
			return nil, 0, fmt.Errorf("could not find data chunk in WAV file")
		}
	}

	numChannels := int(header.NumChannels)
	numSamples := int(header.Subchunk2Size) / (2 * numChannels) // 2 bytes per sample per channel
	samples := make([]float64, numSamples)

	// Read buffer
	var sampleVal int16
	for i := 0; i < numSamples; i++ {
		// Read channel 0
		err = binary.Read(file, binary.LittleEndian, &sampleVal)
		if err != nil {
			if err == io.EOF {
				// Unexpected EOF, but we'll return what we read so far
				return samples[:i], int(header.SampleRate), nil
			}
			return nil, 0, fmt.Errorf("failed to read audio data: %w", err)
		}
		samples[i] = float64(sampleVal) / 32767.0

		// Skip other channels if multi-channel (we only keep mono)
		for c := 1; c < numChannels; c++ {
			var dummy int16
			_ = binary.Read(file, binary.LittleEndian, &dummy)
		}
	}

	return samples, int(header.SampleRate), nil
}

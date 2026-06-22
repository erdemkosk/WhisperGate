package wav

import (
	"math"
	"os"
	"testing"
)

func TestWriteReadWAV(t *testing.T) {
	tempFile := "test_temp.wav"
	defer os.Remove(tempFile)

	// Generate a 200 ms 440 Hz sine wave
	sampleRate := 44100
	duration := 0.2
	numSamples := int(float64(sampleRate) * duration)
	originalSamples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		originalSamples[i] = math.Sin(2.0 * math.Pi * 440.0 * float64(i) / float64(sampleRate))
	}

	err := WriteWAV(tempFile, originalSamples, sampleRate)
	if err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	readSamples, readRate, err := ReadWAV(tempFile)
	if err != nil {
		t.Fatalf("ReadWAV failed: %v", err)
	}

	if readRate != sampleRate {
		t.Errorf("Sample rate mismatch. Expected %d, got %d", sampleRate, readRate)
	}

	if len(readSamples) != len(originalSamples) {
		t.Fatalf("Sample length mismatch. Expected %d, got %d", len(originalSamples), len(readSamples))
	}

	// Verify values are close (within quantization noise limit of 16-bit int)
	for i := 0; i < len(originalSamples); i++ {
		diff := math.Abs(originalSamples[i] - readSamples[i])
		if diff > (1.0/32767.0 + 1e-4) {
			t.Errorf("Sample %d mismatch. Original %v, read %v, diff %v", i, originalSamples[i], readSamples[i], diff)
		}
	}
}

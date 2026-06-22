package modem

import (
	"bytes"
	"testing"
)

func TestModulateDemodulateNoiseless(t *testing.T) {
	message := []byte("Modem Test!")
	sampleRate := 44100
	baudRate := 50
	f0 := 18000.0
	f1 := 19000.0

	samples := Modulate(message, sampleRate, baudRate, f0, f1)

	decoded, err := Demodulate(samples, sampleRate, baudRate, f0, f1)
	if err != nil {
		t.Fatalf("Demodulate failed: %v", err)
	}

	if !bytes.Equal(decoded, message) {
		t.Errorf("Decoded content mismatch. Expected %q, got %q", string(message), string(decoded))
	}
}

func TestDemodulateWithOffset(t *testing.T) {
	message := []byte("Align")
	sampleRate := 44100
	baudRate := 50
	f0 := 18000.0
	f1 := 19000.0

	samples := Modulate(message, sampleRate, baudRate, f0, f1)

	// Add 300 samples of silence (zero padding) at the beginning to test timing alignment detection
	paddedSamples := make([]float64, 300+len(samples))
	copy(paddedSamples[300:], samples)

	decoded, err := Demodulate(paddedSamples, sampleRate, baudRate, f0, f1)
	if err != nil {
		t.Fatalf("Demodulate with padding failed: %v", err)
	}

	if !bytes.Equal(decoded, message) {
		t.Errorf("Decoded content mismatch with offset. Expected %q, got %q", string(message), string(decoded))
	}
}

func TestDemodulateStream(t *testing.T) {
	message := []byte("StreamTest")
	sampleRate := 44100
	baudRate := 50
	f0 := 18000.0
	f1 := 19000.0

	samples := Modulate(message, sampleRate, baudRate, f0, f1)

	// Pad samples with silence at start and end
	paddedSamples := make([]float64, 500+len(samples)+1000)
	copy(paddedSamples[500:], samples)

	frame, err := DemodulateStream(paddedSamples, sampleRate, baudRate, f0, f1)
	if err != nil {
		t.Fatalf("DemodulateStream failed: %v", err)
	}

	if !bytes.Equal(frame.Payload, message) {
		t.Errorf("Expected payload %q, got %q", string(message), string(frame.Payload))
	}

	// Verify sample end index
	expectedEnd := 500 + len(samples)
	diff := expectedEnd - frame.SampleEnd
	if diff < 0 {
		diff = -diff
	}
	// Goertzel and CRC32 are highly robust and can decode successfully even with
	// alignment offset up to ~40% of a bit. Thus, we allow a difference of up to half a bit.
	maxDiff := (sampleRate / baudRate) / 2
	if diff > maxDiff {
		t.Errorf("Sample end index mismatch. Expected close to %d, got %d (diff %d, max allowed %d)", expectedEnd, frame.SampleEnd, diff, maxDiff)
	}
}


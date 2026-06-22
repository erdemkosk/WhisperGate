package modem

import (
	"whispergate/dsp"
	"whispergate/protocol"
	"fmt"
	"math"
)

// Modulate converts raw payload bytes into a slice of audio samples.
// It packs the payload into a frame, converts it to bits, and modulates
// those bits into sine wave samples using CPFSK (Continuous Phase Frequency Shift Keying).
func Modulate(payload []byte, sampleRate int, baudRate int, f0 float64, f1 float64) []float64 {
	bits := protocol.Pack(payload)
	samplesPerBit := sampleRate / baudRate

	// Total samples needed
	totalSamples := len(bits) * samplesPerBit
	samples := make([]float64, totalSamples)

	phase := 0.0
	tStep := 1.0 / float64(sampleRate)

	for bitIdx, bit := range bits {
		freq := f0
		if bit {
			freq = f1
		}

		omega := 2.0 * math.Pi * freq
		startIdx := bitIdx * samplesPerBit

		for i := 0; i < samplesPerBit; i++ {
			// Continuous phase synthesis to avoid click/pop sounds
			phase += omega * tStep
			if phase > 2.0*math.Pi {
				phase -= 2.0 * math.Pi
			}
			samples[startIdx+i] = math.Sin(phase)
		}
	}

	// Apply a global envelope fade-in/fade-out at the beginning and end of transmission
	fadeSamples := 100 // ~2.2 ms at 44.1kHz
	if len(samples) > fadeSamples*2 {
		for i := 0; i < fadeSamples; i++ {
			gain := float64(i) / float64(fadeSamples)
			samples[i] *= gain
			samples[len(samples)-1-i] *= gain
		}
	}

	return samples
}

// Demodulate searches the samples for a valid frame by testing different bit offsets.
// Returns the first decoded payload, or an error if no valid frames are found.
func Demodulate(samples []float64, sampleRate int, baudRate int, f0 float64, f1 float64) ([]byte, error) {
	payloads := DemodulateAll(samples, sampleRate, baudRate, f0, f1)
	if len(payloads) > 0 {
		return payloads[0], nil
	}
	return nil, fmt.Errorf("failed to demodulate: no valid frames found (CRC or Preamble mismatch)")
}

// DemodulateAll scans the samples under multiple offset candidates and extracts all valid payloads.
func DemodulateAll(samples []float64, sampleRate int, baudRate int, f0 float64, f1 float64) [][]byte {
	samplesPerBit := sampleRate / baudRate
	if len(samples) < samplesPerBit {
		return nil
	}

	// Test offsets in steps to find the optimal bit-boundary alignment
	step := 20
	if step > samplesPerBit/4 {
		step = 1
	}

	for offset := 0; offset < samplesPerBit; offset += step {
		bits := DecodeBits(samples, sampleRate, baudRate, f0, f1, offset)
		payloads := protocol.ExtractAllFrames(bits)
		if len(payloads) > 0 {
			return payloads
		}
	}

	return nil
}

// DecodeBits extracts raw bits from samples assuming a specific start offset.
func DecodeBits(samples []float64, sampleRate int, baudRate int, f0 float64, f1 float64, offset int) []bool {
	samplesPerBit := sampleRate / baudRate
	numBits := (len(samples) - offset) / samplesPerBit
	if numBits <= 0 {
		return nil
	}
	bits := make([]bool, numBits)

	for i := 0; i < numBits; i++ {
		start := offset + i*samplesPerBit
		end := start + samplesPerBit
		bitSamples := samples[start:end]

		power0 := dsp.GoertzelPower(bitSamples, f0, float64(sampleRate))
		power1 := dsp.GoertzelPower(bitSamples, f1, float64(sampleRate))

		bits[i] = power1 > power0
	}

	return bits
}

// StreamFrame represents a decoded frame from an audio stream.
type StreamFrame struct {
	Payload   []byte
	SampleEnd int
}

// DemodulateStream scans the input samples under candidate timing offsets to locate and decode a frame.
// Returns the first decoded frame including the sample end index, or an error if none is found.
func DemodulateStream(samples []float64, sampleRate int, baudRate int, f0 float64, f1 float64) (StreamFrame, error) {
	samplesPerBit := sampleRate / baudRate
	if len(samples) < samplesPerBit {
		return StreamFrame{}, fmt.Errorf("insufficient samples")
	}

	step := 20
	if step > samplesPerBit/4 {
		step = 1
	}

	for offset := 0; offset < samplesPerBit; offset += step {
		bits := DecodeBits(samples, sampleRate, baudRate, f0, f1, offset)
		frames := protocol.ExtractFramesWithBounds(bits)
		if len(frames) > 0 {
			firstFrame := frames[0]
			sampleEnd := offset + firstFrame.BitEnd*samplesPerBit
			return StreamFrame{
				Payload:   firstFrame.Payload,
				SampleEnd: sampleEnd,
			}, nil
		}
	}

	return StreamFrame{}, fmt.Errorf("no frames found")
}


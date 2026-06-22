package dsp

import (
	"math"
	"testing"
)

func TestGoertzelPower(t *testing.T) {
	sampleRate := 44100.0
	f0 := 18000.0
	f1 := 19000.0
	numSamples := 882 // 20 ms

	// Generate a pure 18 kHz sine wave
	samplesF0 := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		samplesF0[i] = math.Sin(2.0 * math.Pi * f0 * float64(i) / sampleRate)
	}

	power0_f0 := GoertzelPower(samplesF0, f0, sampleRate)
	power1_f0 := GoertzelPower(samplesF0, f1, sampleRate)

	if power0_f0 <= power1_f0 {
		t.Errorf("Expected 18kHz to be dominant, got power0=%v, power1=%v", power0_f0, power1_f0)
	}

	// Generate a pure 19 kHz sine wave
	samplesF1 := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		samplesF1[i] = math.Sin(2.0 * math.Pi * f1 * float64(i) / sampleRate)
	}

	power0_f1 := GoertzelPower(samplesF1, f0, sampleRate)
	power1_f1 := GoertzelPower(samplesF1, f1, sampleRate)

	if power1_f1 <= power0_f1 {
		t.Errorf("Expected 19kHz to be dominant, got power0=%v, power1=%v", power0_f1, power1_f1)
	}
}

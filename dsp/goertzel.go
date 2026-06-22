package dsp

import (
	"math"
)

// GoertzelPower calculates the power (magnitude squared) of a target frequency
// in a slice of audio samples at a given sample rate using the Goertzel algorithm.
func GoertzelPower(samples []float64, targetFreq float64, sampleRate float64) float64 {
	n := len(samples)
	if n == 0 {
		return 0.0
	}

	// Calculate coefficient
	omega := 2.0 * math.Pi * targetFreq / sampleRate
	coeff := 2.0 * math.Cos(omega)

	// Goertzel states
	sPrev := 0.0
	sPrev2 := 0.0

	for _, sample := range samples {
		s := sample + coeff*sPrev - sPrev2
		sPrev2 = sPrev
		sPrev = s
	}

	// Calculate magnitude squared
	power := sPrev*sPrev + sPrev2*sPrev2 - coeff*sPrev*sPrev2
	
	// Normalize by N^2 to get normalized power independent of block size
	return power / float64(n*n)
}

package audio

import (
	"sync"

	"github.com/gordonklaus/portaudio"
)

// AudioSystem manages PortAudio duplex streams and thread-safe queues.
type AudioSystem struct {
	stream     *portaudio.Stream
	sampleRate int

	// Captured microphone samples
	inputBuf []float64
	inputMu  sync.Mutex

	// Modulated playback samples
	outputBuf []float32
	outputMu  sync.Mutex
}

// NewAudioSystem initializes PortAudio and opens the default duplex stream.
func NewAudioSystem(sampleRate int) (*AudioSystem, error) {
	err := portaudio.Initialize()
	if err != nil {
		return nil, err
	}

	as := &AudioSystem{
		sampleRate: sampleRate,
	}

	// Open duplex stream: 1 input channel, 1 output channel
	stream, err := portaudio.OpenDefaultStream(1, 1, float64(sampleRate), 1024, as.audioCallback)
	if err != nil {
		portaudio.Terminate()
		return nil, err
	}

	as.stream = stream
	return as, nil
}

// Start activates the audio stream.
func (as *AudioSystem) Start() error {
	return as.stream.Start()
}

// Stop stops and releases PortAudio resources.
func (as *AudioSystem) Stop() error {
	as.stream.Stop()
	as.stream.Close()
	return portaudio.Terminate()
}

// audioCallback is invoked by PortAudio to process input and output buffers concurrently.
func (as *AudioSystem) audioCallback(in, out []float32) {
	as.outputMu.Lock()
	// Check if we are currently transmitting (playing out modulated sound)
	isTransmitting := len(as.outputBuf) > 0

	n := copy(out, as.outputBuf)
	if n < len(out) {
		// Out of playback samples, fill remainder with silence
		for i := n; i < len(out); i++ {
			out[i] = 0
		}
		as.outputBuf = nil
	} else {
		as.outputBuf = as.outputBuf[n:]
	}
	as.outputMu.Unlock()

	// Half-Duplex Echo Suppression:
	// If transmitting, we discard the microphone samples so we don't hear ourselves.
	if !isTransmitting {
		as.inputMu.Lock()
		for _, v := range in {
			as.inputBuf = append(as.inputBuf, float64(v))
		}
		as.inputMu.Unlock()
	}
}

// Play queues modulated sound samples for output.
func (as *AudioSystem) Play(samples []float64) {
	as.outputMu.Lock()
	defer as.outputMu.Unlock()

	samples32 := make([]float32, len(samples))
	for i, v := range samples {
		samples32[i] = float32(v)
	}
	as.outputBuf = append(as.outputBuf, samples32...)
}

// GetInputSamples drains the collected microphone samples from the queue.
func (as *AudioSystem) GetInputSamples() []float64 {
	as.inputMu.Lock()
	defer as.inputMu.Unlock()

	if len(as.inputBuf) == 0 {
		return nil
	}

	res := make([]float64, len(as.inputBuf))
	copy(res, as.inputBuf)
	as.inputBuf = nil
	return res
}

// IsTransmitting returns true if there is pending audio to be played.
func (as *AudioSystem) IsTransmitting() bool {
	as.outputMu.Lock()
	defer as.outputMu.Unlock()
	return len(as.outputBuf) > 0
}

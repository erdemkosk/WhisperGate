package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"whispergate/audio"
	"whispergate/dsp"
	"whispergate/modem"
	"whispergate/wav"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	// Seed random generator for test noise
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 {
		// Default to Chat TUI if no subcommands are provided
		runChatTUI()
		os.Exit(0)
	}

	command := os.Args[1]
	if len(command) > 0 && command[0] == '-' {
		// If the first argument is a flag, default to Chat TUI
		runChatTUI()
		os.Exit(0)
	}

	switch command {
	case "chat":
		runChatTUI()
	case "encode":
		runEncode()
	case "decode":
		runDecode()
	case "test":
		runTest()
	default:
		fmt.Printf("\033[31mUnknown command: %s\033[0m\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf("\033[36m\033[1m=== WhisperGate - Help ===\033[0m\n")
	fmt.Println("A peer-to-peer acoustic modem chat client and simulator working over sound waves.")
	fmt.Println("\nUsage:")
	fmt.Println("  whispergate               Start the interactive real-time Chat TUI (Default)")
	fmt.Println("  whispergate chat          Start the interactive real-time Chat TUI")
	fmt.Println("  whispergate encode [...]  Convert a text message into a modulated WAV file.")
	fmt.Println("  whispergate decode [...]  Read a WAV file and demodulate it back to a text message.")
	fmt.Println("  whispergate test          Run a series of automated end-to-end tests including noise injection.")
	fmt.Println("")
}

func runChatTUI() {
	chatCmd := flag.NewFlagSet("chat", flag.ExitOnError)
	rateOpt := chatCmd.Int("samplerate", 44100, "Audio sample rate in Hz")
	baudOpt := chatCmd.Int("baud", 50, "Baud rate (bits per second)")
	f0Opt := chatCmd.Float64("f0", 18000, "Frequency for bit 0 in Hz")
	f1Opt := chatCmd.Float64("f1", 19000, "Frequency for bit 1 in Hz")

	// Parse flags depending on whether we came from main with or without "chat" subcommand
	var args []string
	if len(os.Args) >= 2 && os.Args[1] == "chat" {
		args = os.Args[2:]
	} else if len(os.Args) >= 1 && os.Args[0] != "" {
		args = os.Args[1:]
	}
	_ = chatCmd.Parse(args)

	sampleRate := *rateOpt
	baudRate := *baudOpt
	f0 := *f0Opt
	f1 := *f1Opt

	// Initialize Audio System
	as, err := audio.NewAudioSystem(sampleRate)
	if err != nil {
		fmt.Printf("\033[31mError initializing audio system: %v\033[0m\n", err)
		fmt.Println("Ensure microphone permissions are enabled for your terminal client.")
		os.Exit(1)
	}
	defer as.Stop()

	err = as.Start()
	if err != nil {
		fmt.Printf("\033[31mError starting audio stream: %v\033[0m\n", err)
		os.Exit(1)
	}

	app := tview.NewApplication()

	// Chat History View
	historyView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	historyView.SetBorder(true).
		SetTitle(" WhisperGate - Chat History ").
		SetBorderPadding(1, 1, 1, 1)

	// Welcome message
	welcomeMsg := fmt.Sprintf("[#ffff00][System] Welcome to WhisperGate Chat![-]\n"+
		"[#ffff00][System] Modulation: CPFSK | Baud: %d bps | F0/F1: %.1f/%.1f Hz[-]\n"+
		"[#ffff00][System] Type message below and press Enter to transmit. Press Ctrl+C to exit.[-]\n"+
		"--------------------------------------------------------------------------------\n\n",
		baudRate, f0, f1)
	fmt.Fprint(historyView, welcomeMsg)

	// Status View
	statusView := tview.NewTextView().SetDynamicColors(true)
	statusView.SetText(fmt.Sprintf("[#55ff55][STATUS: LISTENING...][#ffffff]    | Baud: %d bps | F0/F1: %.1f/%.1f Hz", baudRate, f0, f1))

	// Message Input Field
	inputField := tview.NewInputField().
		SetLabel(" Message > ").
		SetFieldWidth(0).
		SetLabelColor(tcell.GetColor("cyan"))

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := inputField.GetText()
			if len(text) == 0 {
				return
			}
			if len(text) > 100 {
				timestamp := time.Now().Format("15:04:05")
				fmt.Fprintf(historyView, "[%s] [#ff5555][System] Message too long (max 100 chars).[-]\n", timestamp)
				historyView.ScrollToEnd()
				return
			}

			// Modulate and play via Speaker
			samples := modem.Modulate([]byte(text), sampleRate, baudRate, f0, f1)
			as.Play(samples)

			// Display local message in history
			timestamp := time.Now().Format("15:04:05")
			fmt.Fprintf(historyView, "[%s] [#00ffff][Me][#ffffff]    %s\n", timestamp, text)
			historyView.ScrollToEnd()

			inputField.SetText("")
		}
	})

	// Layout Flex
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(historyView, 0, 1, false).
		AddItem(inputField, 1, 0, true).
		AddItem(statusView, 1, 0, false)

	// Start background demodulation scanner
	go startDemodulator(as, app, historyView, statusView, sampleRate, baudRate, f0, f1)

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		fmt.Printf("TUI Error: %v\n", err)
		os.Exit(1)
	}
}

func startDemodulator(as *audio.AudioSystem, app *tview.Application, history *tview.TextView, status *tview.TextView, sampleRate, baudRate int, f0, f1 float64) {
	var rollingBuffer []float64

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// Read raw mic samples from sound card queue
		newSamples := as.GetInputSamples()
		if len(newSamples) > 0 {
			rollingBuffer = append(rollingBuffer, newSamples...)
		}

		// Prevent memory leaks: cap buffer at 45 seconds of audio
		maxLen := 45 * sampleRate
		if len(rollingBuffer) > maxLen {
			rollingBuffer = rollingBuffer[len(rollingBuffer)-maxLen:]
		}

		// Demodulate next packet if present
		frame, err := modem.DemodulateStream(rollingBuffer, sampleRate, baudRate, f0, f1)
		if err == nil {
			// Extract SNR of the detected packet portion
			snr := calculateSNREstimate(rollingBuffer[:frame.SampleEnd], sampleRate, baudRate, f0, f1)

			// Clean decoded samples from buffer
			rollingBuffer = rollingBuffer[frame.SampleEnd:]

			timestamp := time.Now().Format("15:04:05")
			msg := fmt.Sprintf("[%s] [#55ff55][Peer][#ffffff]  %s   [#ffff00](SNR: %.1f dB)[-]\n", timestamp, string(frame.Payload), snr)

			app.QueueUpdateDraw(func() {
				fmt.Fprint(history, msg)
				history.ScrollToEnd()
			})
		}

		// Refresh Status pane
		app.QueueUpdateDraw(func() {
			if as.IsTransmitting() {
				status.SetText(fmt.Sprintf("[#ff5555][STATUS: TRANSMITTING...][#ffffff] | Baud: %d bps | F0/F1: %.1f/%.1f Hz", baudRate, f0, f1))
			} else {
				status.SetText(fmt.Sprintf("[#55ff55][STATUS: LISTENING...][#ffffff]    | Baud: %d bps | F0/F1: %.1f/%.1f Hz", baudRate, f0, f1))
			}
		})
	}
}

func runEncode() {
	encodeCmd := flag.NewFlagSet("encode", flag.ExitOnError)
	msgOpt := encodeCmd.String("message", "Merhaba Dunya!", "Message to modulate")
	outOpt := encodeCmd.String("out", "transfer.wav", "Output WAV file path")
	rateOpt := encodeCmd.Int("samplerate", 44100, "Audio sample rate in Hz")
	baudOpt := encodeCmd.Int("baud", 50, "Baud rate (bits per second)")
	f0Opt := encodeCmd.Float64("f0", 18000, "Frequency for bit 0 in Hz")
	f1Opt := encodeCmd.Float64("f1", 19000, "Frequency for bit 1 in Hz")

	_ = encodeCmd.Parse(os.Args[2:])

	fmt.Printf("\033[36m\033[1m=== WhisperGate - Modulator ===\033[0m\n")
	fmt.Printf("Message:      %q\n", *msgOpt)
	fmt.Printf("Output File:  %s\n", *outOpt)
	fmt.Printf("Sample Rate:  %d Hz\n", *rateOpt)
	fmt.Printf("Baud Rate:    %d bps\n", *baudOpt)
	fmt.Printf("Frequencies:  0 = %.1f Hz, 1 = %.1f Hz\n", *f0Opt, *f1Opt)
	fmt.Println("---------------------------------")

	samples := modem.Modulate([]byte(*msgOpt), *rateOpt, *baudOpt, *f0Opt, *f1Opt)

	duration := float64(len(samples)) / float64(*rateOpt)
	fmt.Printf("Generated \033[32m%d\033[0m samples (Duration: \033[32m%.2f\033[0m seconds)\n", len(samples), duration)

	err := wav.WriteWAV(*outOpt, samples, *rateOpt)
	if err != nil {
		fmt.Printf("\033[31mError writing WAV file: %v\033[0m\n", err)
		os.Exit(1)
	}

	fmt.Printf("\033[32m\033[1mSuccess! File saved to %s\033[0m\n", *outOpt)
}

func runDecode() {
	decodeCmd := flag.NewFlagSet("decode", flag.ExitOnError)
	fileOpt := decodeCmd.String("file", "transfer.wav", "WAV file path to decode")
	rateOpt := decodeCmd.Int("samplerate", 44100, "Audio sample rate in Hz")
	baudOpt := decodeCmd.Int("baud", 50, "Baud rate (bits per second)")
	f0Opt := decodeCmd.Float64("f0", 18000, "Frequency for bit 0 in Hz")
	f1Opt := decodeCmd.Float64("f1", 19000, "Frequency for bit 1 in Hz")

	_ = decodeCmd.Parse(os.Args[2:])

	fmt.Printf("\033[36m\033[1m=== WhisperGate - Demodulator ===\033[0m\n")
	fmt.Printf("Input File:   %s\n", *fileOpt)
	fmt.Printf("Sample Rate:  %d Hz\n", *rateOpt)
	fmt.Printf("Baud Rate:    %d bps\n", *baudOpt)
	fmt.Printf("Frequencies:  0 = %.1f Hz, 1 = %.1f Hz\n", *f0Opt, *f1Opt)
	fmt.Println("---------------------------------")

	samples, wavRate, err := wav.ReadWAV(*fileOpt)
	if err != nil {
		fmt.Printf("\033[31mError reading WAV file: %v\033[0m\n", err)
		os.Exit(1)
	}

	if wavRate != *rateOpt {
		fmt.Printf("\033[33mWarning: WAV sample rate (%d Hz) differs from configured sample rate (%d Hz). Using WAV rate.\033[0m\n", wavRate, *rateOpt)
		*rateOpt = wavRate
	}

	fmt.Printf("Loaded %d samples from file.\n", len(samples))
	fmt.Println("Demodulating and analyzing signal...")

	payload, err := modem.Demodulate(samples, *rateOpt, *baudOpt, *f0Opt, *f1Opt)
	if err != nil {
		fmt.Printf("\033[31mDemodulation failed: %v\033[0m\n", err)
		os.Exit(1)
	}

	snr := calculateSNREstimate(samples, *rateOpt, *baudOpt, *f0Opt, *f1Opt)

	fmt.Println("---------------------------------")
	fmt.Printf("\033[32m\033[1mDecoded Message:\033[0m %s\n", string(payload))
	fmt.Printf("Signal Quality (SNR Estimate): \033[32m%.1f dB\033[0m\n", snr)
}

func runTest() {
	fmt.Printf("\033[36m\033[1m=== WhisperGate - End-to-End Simulation Tests ===\033[0m\n\n")

	testMessage := "Merhaba Dunya! WhisperGate test ediliyor."
	sampleRate := 44100
	baudRate := 50
	f0 := 18000.0
	f1 := 19000.0

	fmt.Printf("Test Payload:      %q\n", testMessage)
	fmt.Printf("Modulation Setup:  %d Hz sample rate, %d bps baud, 18/19 kHz\n", sampleRate, baudRate)
	fmt.Println("----------------------------------------------------------------------")

	// 1. Modulate
	fmt.Printf("Step 1: Modulating message... ")
	samples := modem.Modulate([]byte(testMessage), sampleRate, baudRate, f0, f1)
	fmt.Printf("\033[32mOK\033[0m (%d samples generated)\n", len(samples))

	// 2. Test Noiseless
	fmt.Printf("Step 2: Noiseless Demodulation... ")
	decoded, err := modem.Demodulate(samples, sampleRate, baudRate, f0, f1)
	if err != nil {
		fmt.Printf("\033[31mFAILED: %v\033[0m\n", err)
	} else if string(decoded) != testMessage {
		fmt.Printf("\033[31mFAILED: content mismatch. Got %q\033[0m\n", string(decoded))
	} else {
		snr := calculateSNREstimate(samples, sampleRate, baudRate, f0, f1)
		fmt.Printf("\033[32mPASSED\033[0m (SNR: %.1f dB)\n", snr)
	}

	// 3. Test Attenuation (low amplitude)
	fmt.Printf("Step 3: Attenuated Signal (-20 dB amplitude)... ")
	attenuatedSamples := make([]float64, len(samples))
	for i, s := range samples {
		attenuatedSamples[i] = s * 0.1 // 10% amplitude
	}
	decoded, err = modem.Demodulate(attenuatedSamples, sampleRate, baudRate, f0, f1)
	if err != nil {
		fmt.Printf("\033[31mFAILED: %v\033[0m\n", err)
	} else if string(decoded) != testMessage {
		fmt.Printf("\033[31mFAILED: content mismatch\033[0m\n")
	} else {
		snr := calculateSNREstimate(attenuatedSamples, sampleRate, baudRate, f0, f1)
		fmt.Printf("\033[32mPASSED\033[0m (SNR: %.1f dB)\n", snr)
	}

	// 4. Test Noise Injection (AWGN) at different SNR levels
	snrLevels := []struct {
		db     float64
		stdDev float64
	}{
		{20.0, 0.0707},
		{10.0, 0.2236},
		{6.0, 0.3540},
		{3.0, 0.5000},
	}

	fmt.Println("\nStep 4: Testing AWGN (Additive White Gaussian Noise) Robustness:")
	for _, lv := range snrLevels {
		fmt.Printf("  Target SNR: %2.0fdB (Noise StdDev: %.4f) ... ", lv.db, lv.stdDev)
		noisySamples := addNoise(samples, lv.stdDev)

		decoded, err = modem.Demodulate(noisySamples, sampleRate, baudRate, f0, f1)
		if err != nil {
			fmt.Printf("\033[31mFAILED (CRC check failed or preamble missed)\033[0m\n")
		} else if string(decoded) != testMessage {
			fmt.Printf("\033[31mFAILED (Corrupted decoding)\033[0m\n")
		} else {
			measuredSNR := calculateSNREstimate(noisySamples, sampleRate, baudRate, f0, f1)
			fmt.Printf("\033[32mPASSED\033[0m (Measured SNR: %.1f dB)\n", measuredSNR)
		}
	}
	fmt.Println("----------------------------------------------------------------------")
	fmt.Println("\033[32m\033[1mTests complete.\033[0m")
}

func gaussianNoise() float64 {
	u1 := rand.Float64()
	u2 := rand.Float64()
	// Box-Muller transform
	return math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
}

func addNoise(samples []float64, noiseStdDev float64) []float64 {
	noisy := make([]float64, len(samples))
	for i, s := range samples {
		noisy[i] = s + gaussianNoise()*noiseStdDev
	}
	return noisy
}

func calculateSNREstimate(samples []float64, sampleRate int, baudRate int, f0 float64, f1 float64) float64 {
	samplesPerBit := sampleRate / baudRate
	if len(samples) < samplesPerBit {
		return 0.0
	}

	bestOffset := 0
	maxSNR := -999.0
	step := 20

	for offset := 0; offset < samplesPerBit; offset += step {
		numBits := (len(samples) - offset) / samplesPerBit
		if numBits <= 0 {
			continue
		}

		sumRatio := 0.0
		validBlocks := 0

		for i := 0; i < numBits; i++ {
			start := offset + i*samplesPerBit
			end := start + samplesPerBit
			bitSamples := samples[start:end]

			power0 := dsp.GoertzelPower(bitSamples, f0, float64(sampleRate))
			power1 := dsp.GoertzelPower(bitSamples, f1, float64(sampleRate))

			maxP := math.Max(power0, power1)
			minP := math.Min(power0, power1)

			if minP > 0 {
				sumRatio += maxP / minP
				validBlocks++
			}
		}

		if validBlocks > 0 {
			avgRatio := sumRatio / float64(validBlocks)
			snrVal := 10.0 * math.Log10(avgRatio)
			if snrVal > maxSNR {
				maxSNR = snrVal
				bestOffset = offset
			}
		}
	}

	_ = bestOffset
	if maxSNR < 0 {
		return 0.0
	}
	if maxSNR > 99.9 {
		return 99.9
	}
	return maxSNR
}

# 📡 WhisperGate

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-macOS-lightgrey?style=flat&logo=apple)](https://apple.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)

**WhisperGate** is a real-time, peer-to-peer (P2P) acoustic communication client and simulator written in pure Go. It enables adjacent computers to exchange encrypted or plain text messages entirely offline over sound waves, using only their built-in speakers and microphones—bridging air-gapped systems using espionage-inspired acoustic sidechannels.

> [!IMPORTANT]
> **No Internet, No WiFi, No Bluetooth, No Cables.** Just raw acoustic waves transmitting data through the air.

> [!WARNING]
> **Disclaimer:**
> This project is currently in an alpha stage and is intended for demonstration and testing purposes only. Due to differences in hardware, audio devices, operating systems, microphone quality, and noise filtering technologies, some computers may not produce certain frequencies correctly, while others may not detect them at all. Results may vary significantly between devices and environments. We do not guarantee consistent functionality, accuracy, or compatibility across all systems. This project is provided as-is for demonstration purposes only.

---

## 🛠️ How It Works (The Big Picture)

```
 TRANSMITTER (Me)
+---------------+      +-------------------+      +-------------------+      +-------------+
|  Text Input   | ---> | Pack (Header/CRC) | ---> | Modulator (CPFSK) | ---> |   Speaker   |
+---------------+      +-------------------+      +-------------------+      +-------------+
                                                                                    |
                                                                              (Sound Waves)
                                                                                    |
  RECEIVER (Peer)                                                                   v
+---------------+      +-------------------+      +-------------------+      +-------------+
|   TUI Chat    | <--- | Unpack & Verify   | <--- | Demod (Goertzel)  | <--- | Microphone  |
+---------------+      +-------------------+      +-------------------+      +-------------+
```

1. **Modulation:** Text is packed, converted into bits, and translated into high-frequency sine waves. WhisperGate uses **CPFSK (Continuous Phase Frequency Shift Keying)** to synthesize frequencies smoothly, eliminating click noises and reducing spectral splatter.
2. **Espionage/Stealth Frequency:** By default, it operates at **$18\text{ kHz}$ (Bit 0)** and **$19\text{ kHz}$ (Bit 1)**. These frequencies are near-ultrasonic—virtually inaudible to human ears but fully captureable by standard laptop microphones.
3. **Demodulation:** The receiver captures audio, feeds it into a rolling sample buffer, and runs the **Goertzel Algorithm** (a highly optimized DSP filter that acts as a narrow-band DFT) to measure frequency energies at 18/19 kHz.
4. **Synchronization & Alignment:** A brute-force offset search aligns the fractional sample boundaries of incoming bits, locking onto a 16-bit preamble sequence.
5. **Validation:** Packets are protected by an IEEE **CRC32 Checksum**. If the decoded checksum matches, the message is instantly drawn to the terminal UI.

---

## 🌟 Key Features

*   🔊 **Real-time Duplex Streaming:** Simultaneous audio capture and playback via Go bindings for PortAudio.
*   🚫 **Half-Duplex Echo Suppression:** A software-level gate that automatically discards microphone samples while the local speaker is playing, preventing the system from hearing and repeating its own transmissions.
*   📐 **Optimized DSP Goertzel Filters:** Ultra-low CPU usage compared to standard Fast Fourier Transforms (FFT).
*   💻 **Interactive Terminal UI (TUI):** A split-screen chat interface displaying color-coded message history, timestamps, transmission status, and real-time **Signal-to-Noise Ratio (SNR)** quality metrics.
*   🧪 **AWGN Stress Test Simulator:** A built-in simulator that injects Additive White Gaussian Noise (AWGN) to benchmark the modem's decoding capability under extreme environmental noise.

---

## ⚙️ Technical Specifications

| Parameter | Default Value | Description |
| :--- | :--- | :--- |
| **Sample Rate** | 44,100 Hz | Standard CD quality, supports frequencies up to 22.05 kHz |
| **Bit '0' Frequency** | 18,000 Hz | Near-ultrasonic carrier frequency representing binary `0` |
| **Bit '1' Frequency** | 19,000 Hz | Near-ultrasonic carrier frequency representing binary `1` |
| **Baud Rate** | 50 bps | 20 ms per bit duration (882 audio samples per bit) |
| **Preamble Sequence**| `0xAB35` | 16-bit sync word (`10101011 00110101`) to align bit windows |
| **Checksum** | CRC32 IEEE | 4-byte polynomial checking for transmission errors |

---

## 🚀 Installation & Build

WhisperGate requires `portaudio` and `pkg-config` to compile CGO audio bindings on macOS.

### 1. Install System Dependencies
Install PortAudio and PkgConfig via [Homebrew](https://brew.sh):
```bash
brew install portaudio pkg-config
```

### 2. Download Dependencies
```bash
go mod download
```

### 3. Build the Executable
```bash
go build -o whispergate
```

---

## 📖 Usage Guide

WhisperGate supports both live interactive chat and offline command utility workflows.

### 1. Interactive Chat TUI (Default Mode)
Start the chat application in silent/stealth mode ($18\text{ kHz}$ / $19\text{ kHz}$):
```bash
./whispergate
```

To run with **audible frequencies** (ideal for testing, troubleshooting, or just hearing the nostalgic modem sounds):
```bash
./whispergate -f0 1000 -f1 1500
```
> [!TIP]
> This configures the modem to use $1,000\text{ Hz}$ (binary 0) and $1,500\text{ Hz}$ (binary 1), which are highly audible to humans.

### 2. Scientific AWGN Robustness Test
Stress-test the CPFSK modulator and Goertzel demodulator against simulated Gaussian background noise:
```bash
./whispergate test
```
*Outputs verification reports across $20\text{ dB}$, $10\text{ dB}$, $6\text{ dB}$, and $3\text{ dB}$ SNR levels.*

### 3. Modulate Message to WAV File
Translate any string into a 16-bit PCM WAV file:
```bash
./whispergate encode -message "Hello Agent" -out secret.wav
```

### 4. Demodulate Message from WAV File
Read and decode any WhisperGate WAV file:
```bash
./whispergate decode -file secret.wav
```

---

## 🛡️ License
Distributed under the MIT License. See `LICENSE` for more information.

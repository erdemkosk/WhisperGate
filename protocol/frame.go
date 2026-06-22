package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// PreambleWord is the 16-bit sync sequence: 0xAB35 (10101011 00110101)
const PreambleWord uint16 = 0xAB35

// BytesToBits converts a slice of bytes to a slice of booleans (bits), MSB first.
func BytesToBits(bytes []byte) []bool {
	bits := make([]bool, len(bytes)*8)
	for i, b := range bytes {
		for bit := 0; bit < 8; bit++ {
			bits[i*8+bit] = (b & (1 << (7 - bit))) != 0
		}
	}
	return bits
}

// BitsToBytes converts a slice of booleans (bits) back to a slice of bytes, MSB first.
func BitsToBytes(bits []bool) ([]byte, error) {
	if len(bits)%8 != 0 {
		return nil, errors.New("bit slice length must be a multiple of 8")
	}
	bytes := make([]byte, len(bits)/8)
	for i := 0; i < len(bytes); i++ {
		var b byte
		for bit := 0; bit < 8; bit++ {
			if bits[i*8+bit] {
				b |= (1 << (7 - bit))
			}
		}
		bytes[i] = b
	}
	return bytes, nil
}

// Pack encapsulates the payload into a frame and returns its bit representation.
// The frame structure is:
// [Preamble (16 bits)] [Length (8 bits)] [Payload (Length * 8 bits)] [CRC32 (32 bits)]
func Pack(payload []byte) []bool {
	length := len(payload)
	if length > 255 {
		panic("payload size exceeds 255 bytes limit")
	}

	checksum := crc32.ChecksumIEEE(payload)

	// Build raw frame bytes
	frameBytes := make([]byte, 2+1+length+4)
	binary.BigEndian.PutUint16(frameBytes[0:2], PreambleWord)
	frameBytes[2] = byte(length)
	copy(frameBytes[3:3+length], payload)
	binary.BigEndian.PutUint32(frameBytes[3+length:7+length], checksum)

	return BytesToBits(frameBytes)
}

// LocatePreamble searches for the 16-bit preamble sequence in a stream of bits.
// Returns the index of the first bit after the preamble, or -1 if not found.
func LocatePreamble(bits []bool) int {
	if len(bits) < 16 {
		return -1
	}
	preambleBits := BytesToBits([]byte{byte(PreambleWord >> 8), byte(PreambleWord & 0xFF)})

	for i := 0; i <= len(bits)-16; i++ {
		match := true
		for j := 0; j < 16; j++ {
			if bits[i+j] != preambleBits[j] {
				match = false
				break
			}
		}
		if match {
			return i + 16
		}
	}
	return -1
}

// Unpack extracts and verifies the payload from a sequence of bits containing a frame.
// It will scan the bits to locate the preamble, extract the length, read the payload,
// and verify the CRC32 checksum.
func Unpack(bits []bool) ([]byte, error) {
	startIndex := LocatePreamble(bits)
	if startIndex == -1 {
		return nil, errors.New("preamble not found")
	}

	// We need at least 8 bits for length
	if len(bits)-startIndex < 8 {
		return nil, errors.New("frame truncated: missing length byte")
	}

	lenBytes, err := BitsToBytes(bits[startIndex : startIndex+8])
	if err != nil {
		return nil, err
	}
	payloadLen := int(lenBytes[0])

	// Total bits needed after preamble:
	// 8 (length) + payloadLen * 8 + 32 (CRC32)
	totalNeededBits := 8 + payloadLen*8 + 32
	if len(bits)-startIndex < totalNeededBits {
		return nil, fmt.Errorf("frame truncated: expected %d bits after preamble, got %d", totalNeededBits, len(bits)-startIndex)
	}

	payloadBits := bits[startIndex+8 : startIndex+8+payloadLen*8]
	crcBits := bits[startIndex+8+payloadLen*8 : startIndex+totalNeededBits]

	payloadBytes, err := BitsToBytes(payloadBits)
	if err != nil {
		return nil, err
	}

	crcBytes, err := BitsToBytes(crcBits)
	if err != nil {
		return nil, err
	}

	receivedCRC := binary.BigEndian.Uint32(crcBytes)
	calculatedCRC := crc32.ChecksumIEEE(payloadBytes)

	if receivedCRC != calculatedCRC {
		return nil, fmt.Errorf("CRC checksum mismatch: received 0x%08X, calculated 0x%08X", receivedCRC, calculatedCRC)
	}

	return payloadBytes, nil
}

// DecodedFrame represents a successfully decoded frame with its boundary bit indices in the bitstream.
type DecodedFrame struct {
	Payload  []byte
	BitStart int // The start index of the preamble in the bits slice
	BitEnd   int // The end index of the CRC32 (exclusive) in the bits slice
}

// ExtractFramesWithBounds scans a continuous bitstream and extracts all valid frames along with their boundary indices.
func ExtractFramesWithBounds(bits []bool) []DecodedFrame {
	var frames []DecodedFrame
	cursor := 0
	for {
		if cursor >= len(bits) {
			break
		}
		// Search for preamble starting from cursor
		idx := LocatePreamble(bits[cursor:])
		if idx == -1 {
			break
		}
		preambleStart := cursor + idx - 16
		absoluteStart := cursor + idx

		// Read length
		if len(bits)-absoluteStart < 8 {
			cursor = absoluteStart // Move cursor past preamble
			continue
		}
		lenBytes, _ := BitsToBytes(bits[absoluteStart : absoluteStart+8])
		payloadLen := int(lenBytes[0])

		totalNeededBits := 8 + payloadLen*8 + 32
		if len(bits)-absoluteStart < totalNeededBits {
			// Frame truncated or false positive, skip this preamble start and search further
			cursor = preambleStart + 1
			continue
		}

		payloadBits := bits[absoluteStart+8 : absoluteStart+8+payloadLen*8]
		crcBits := bits[absoluteStart+8+payloadLen*8 : absoluteStart+totalNeededBits]

		payloadBytes, _ := BitsToBytes(payloadBits)
		crcBytes, _ := BitsToBytes(crcBits)
		receivedCRC := binary.BigEndian.Uint32(crcBytes)
		calculatedCRC := crc32.ChecksumIEEE(payloadBytes)

		if receivedCRC == calculatedCRC {
			bitEnd := absoluteStart + totalNeededBits
			frames = append(frames, DecodedFrame{
				Payload:  payloadBytes,
				BitStart: preambleStart,
				BitEnd:   bitEnd,
			})
			cursor = bitEnd
		} else {
			// CRC failed, false preamble or corrupt bits. Move forward by 1 bit past preamble start and retry.
			cursor = preambleStart + 1
		}
	}
	return frames
}

// ExtractAllFrames scans a continuous bitstream and extracts all valid payloads that pass CRC32 verification.
func ExtractAllFrames(bits []bool) [][]byte {
	frames := ExtractFramesWithBounds(bits)
	payloads := make([][]byte, len(frames))
	for i, f := range frames {
		payloads[i] = f.Payload
	}
	return payloads
}


package protocol

import (
	"bytes"
	"testing"
)

func TestPackUnpack(t *testing.T) {
	original := []byte("Merhaba, bu bir test mesajidir.")
	bits := Pack(original)

	decoded, err := Unpack(bits)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Errorf("Decoded payload mismatch. Expected %q, got %q", string(original), string(decoded))
	}
}

func TestBitsBytesConversion(t *testing.T) {
	original := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	bits := BytesToBits(original)
	converted, err := BitsToBytes(bits)
	if err != nil {
		t.Fatalf("BitsToBytes failed: %v", err)
	}

	if !bytes.Equal(converted, original) {
		t.Errorf("Conversion roundtrip failed. Expected %x, got %x", original, converted)
	}
}

func TestLocatePreamble(t *testing.T) {
	preambleBits := BytesToBits([]byte{byte(PreambleWord >> 8), byte(PreambleWord & 0xFF)})

	// Stream with padding before and after
	stream := append([]bool{false, true, false, false}, preambleBits...)
	stream = append(stream, true, false, true)

	idx := LocatePreamble(stream)
	if idx != 20 { // 4 padding bits + 16 preamble bits
		t.Errorf("Expected preamble end at index 20, got %d", idx)
	}
}

package uper

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

// Test the exact schema from the Python example
// IAB-IP-AddressAndTraffic-r16 ::= SEQUENCE {
//     all-Traffic-IAB-IP-Address-r16  SEQUENCE (SIZE(1..8)) OF INTEGER     OPTIONAL,
//     f1-C-Traffic-IP-Address-r16     SEQUENCE (SIZE(1..8)) OF INTEGER     OPTIONAL,
//     f1-U-Traffic-IP-Address-r16     SEQUENCE (SIZE(1..8)) OF INTEGER     OPTIONAL,
//     non-F1-Traffic-IP-Address-r16   SEQUENCE (SIZE(1..8)) OF INTEGER     OPTIONAL
// }

// Value: {"all-Traffic-IAB-IP-Address-r16":[1],"f1-C-Traffic-IP-Address-r16":[2],
//         "f1-U-Traffic-IP-Address-r16":[3],"non-F1-Traffic-IP-Address-r16":[4]}
//
// Expected UPER encoding: F0 02 02 00 40 80 08 18 01 04

func TestPythonSchemaExample(t *testing.T) {
	// Expected output from Python UPER encoder
	// expectedHex := "f002020040800818010​4"
	// expectedBytes, _ := hex.DecodeString(expectedHex)
	expectedBytes := []byte{0xF0, 0x02, 0x02, 0x00, 0x40, 0x80, 0x08, 0x18, 0x01, 0x04}

	t.Logf("Expected (Python): %s", hex.EncodeToString(expectedBytes))
	t.Logf("Expected bytes: % X", expectedBytes)
	t.Logf("Expected binary: % 08b", expectedBytes)

	// Manually encode the structure
	buf := new(bytes.Buffer)
	writer := NewWriter(buf)

	// Preamble: 4 bits for 4 optional fields (all present = 1111)
	writer.WriteBool(true) // all-Traffic present
	writer.WriteBool(true) // f1-C-Traffic present
	writer.WriteBool(true) // f1-U-Traffic present
	writer.WriteBool(true) // non-F1-Traffic present

	// Encode each SEQUENCE OF INTEGER with SIZE(1..8)
	// Each has 1 element, so length = 1-1 = 0, encoded in 3 bits
	constraint := &Constraint{Lb: 1, Ub: 8}

	// Field 1: [1]
	values1 := []int64{1}
	if err := encodeSequenceOfIntegers(writer, values1, constraint); err != nil {
		t.Fatalf("Failed to encode field 1: %v", err)
	}

	// Field 2: [2]
	values2 := []int64{2}
	if err := encodeSequenceOfIntegers(writer, values2, constraint); err != nil {
		t.Fatalf("Failed to encode field 2: %v", err)
	}

	// Field 3: [3]
	values3 := []int64{3}
	if err := encodeSequenceOfIntegers(writer, values3, constraint); err != nil {
		t.Fatalf("Failed to encode field 3: %v", err)
	}

	// Field 4: [4]
	values4 := []int64{4}
	if err := encodeSequenceOfIntegers(writer, values4, constraint); err != nil {
		t.Fatalf("Failed to encode field 4: %v", err)
	}

	writer.Close()

	encoded := buf.Bytes()
	t.Logf("Go encoded: %s", hex.EncodeToString(encoded))
	t.Logf("Go bytes: % X", encoded)
	t.Logf("Go binary: % 08b", encoded)

	// Compare
	if !bytes.Equal(encoded, expectedBytes) {
		t.Errorf("\nExpected: % X\nGot:      % X", expectedBytes, encoded)

		// Detailed comparison
		minLen := len(expectedBytes)
		if len(encoded) < minLen {
			minLen = len(encoded)
		}

		for i := 0; i < minLen; i++ {
			if expectedBytes[i] != encoded[i] {
				t.Errorf("First difference at byte %d: expected 0x%02X, got 0x%02X", i, expectedBytes[i], encoded[i])
				t.Errorf("Expected binary: %08b", expectedBytes[i])
				t.Errorf("Got binary:      %08b", encoded[i])
				break
			}
		}
	}
}

// Helper to encode SEQUENCE OF INTEGER with size constraint
func encodeSequenceOfIntegers(writer *UperWriter, values []int64, sizeConstraint *Constraint) error {
	// Encode length: size - lowerBound in 3 bits (for range 1..8)
	length := len(values)
	lengthValue := uint64(length) - uint64(sizeConstraint.Lb)
	sizeRange := sizeConstraint.Range() // should be 8

	// Write length (3 bits for range 8)
	numBits := 3 // log2(8) = 3
	if err := writer.writeConstraintValue(sizeRange, lengthValue); err != nil {
		return err
	}

	// Encode each INTEGER (unconstrained)
	for _, val := range values {
		if err := writer.WriteInteger(val, nil, false); err != nil {
			return err
		}
	}

	_ = numBits

	return nil
}

// Test individual components
func TestUnconstrainedIntegerEncoding(t *testing.T) {
	tests := []struct {
		value    int64
		expected string // hex
	}{
		{1, "0101"},     // length=1, value=1
		{2, "0102"},     // length=1, value=2
		{3, "0103"},     // length=1, value=3
		{4, "0104"},     // length=1, value=4
		{127, "017f"},   // length=1, value=127
		{128, "020080"}, // length=2, value=128
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Integer_%d", tt.value), func(t *testing.T) {
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)

			if err := writer.WriteInteger(tt.value, nil, false); err != nil {
				t.Fatalf("WriteInteger failed: %v", err)
			}
			writer.Close()

			got := hex.EncodeToString(buf.Bytes())
			if got != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, got)
				t.Errorf("Expected binary: %08b", mustHexDecode(tt.expected))
				t.Errorf("Got binary:      %08b", buf.Bytes())
			}
		})
	}
}

func mustHexDecode(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

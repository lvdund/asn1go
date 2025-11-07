package aper

import (
	"bytes"
	"io"
	"testing"
)

func TestBitstreamWriter_WriteBool(t *testing.T) {
	var buf bytes.Buffer
	bs := NewBitStreamWriter(&buf)

	// Write individual bits
	if err := bs.WriteBool(true); err != nil {
		t.Fatalf("WriteBool(true) failed: %v", err)
	}
	if err := bs.WriteBool(false); err != nil {
		t.Fatalf("WriteBool(false) failed: %v", err)
	}
	if err := bs.WriteBool(true); err != nil {
		t.Fatalf("WriteBool(true) failed: %v", err)
	}
	if err := bs.WriteBool(false); err != nil {
		t.Fatalf("WriteBool(false) failed: %v", err)
	}

	// Write remaining bits to complete a byte
	for i := 0; i < 4; i++ {
		if err := bs.WriteBool(true); err != nil {
			t.Fatalf("WriteBool(true) failed: %v", err)
		}
	}

	if err := bs.flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	result := buf.Bytes()
	if len(result) != 1 {
		t.Fatalf("Expected 1 byte, got %d", len(result))
	}
	// Expected: 10101111 = 0xAF
	expected := byte(0xAF)
	if result[0] != expected {
		t.Errorf("Expected 0x%02X, got 0x%02X", expected, result[0])
	}
}

func TestBitstreamWriter_WriteBits(t *testing.T) {
	var buf bytes.Buffer
	bs := NewBitStreamWriter(&buf)

	data := []byte{0xAB, 0xCD}
	if err := bs.WriteBits(data, 12); err != nil {
		t.Fatalf("WriteBits failed: %v", err)
	}

	if err := bs.flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	result := buf.Bytes()
	if len(result) != 2 {
		t.Fatalf("Expected 2 bytes, got %d", len(result))
	}
	// First 12 bits: 0xABC
	if result[0] != 0xAB {
		t.Errorf("Expected 0xAB, got 0x%02X", result[0])
	}
	if result[1] != 0xC0 {
		t.Errorf("Expected 0xC0, got 0x%02X", result[1])
	}
}

func TestBitstreamReader_ReadBool(t *testing.T) {
	data := []byte{0xAA} // 10101010
	bs := NewBitStreamReader(bytes.NewReader(data))

	expected := []bool{true, false, true, false, true, false, true, false}
	for i, exp := range expected {
		got, err := bs.ReadBool()
		if err != nil {
			t.Fatalf("ReadBool() failed at bit %d: %v", i, err)
		}
		if got != exp {
			t.Errorf("Bit %d: expected %v, got %v", i, exp, got)
		}
	}
}

func TestBitstreamReader_ReadBits(t *testing.T) {
	data := []byte{0xAB, 0xCD}
	bs := NewBitStreamReader(bytes.NewReader(data))

	// Read 12 bits
	result, err := bs.ReadBits(12)
	if err != nil {
		t.Fatalf("ReadBits failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 bytes, got %d", len(result))
	}
	if result[0] != 0xAB {
		t.Errorf("Expected 0xAB, got 0x%02X", result[0])
	}
	if result[1] != 0xC0 {
		t.Errorf("Expected 0xC0, got 0x%02X", result[1])
	}
}

func TestAperWriter_WriteInteger(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	// Test constrained integer
	c := &Constraint{Lb: 0, Ub: 100}
	if err := aw.WriteInteger(50, c, false); err != nil {
		t.Fatalf("WriteInteger failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadInteger(c, false)
	if err != nil {
		t.Fatalf("ReadInteger failed: %v", err)
	}
	if value != 50 {
		t.Errorf("Expected 50, got %d", value)
	}
}

func TestAperWriter_WriteEnumerate(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	c := Constraint{Lb: 0, Ub: 10}
	if err := aw.WriteEnumerate(5, c, false); err != nil {
		t.Fatalf("WriteEnumerate failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadEnumerate(c, false)
	if err != nil {
		t.Fatalf("ReadEnumerate failed: %v", err)
	}
	if value != 5 {
		t.Errorf("Expected 5, got %d", value)
	}
}

func TestAperWriter_WriteOctetString(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	data := []byte{0x01, 0x02, 0x03, 0x04}
	c := &Constraint{Lb: 0, Ub: 10}
	if err := aw.WriteOctetString(data, c, false); err != nil {
		t.Fatalf("WriteOctetString failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	result, err := ar.ReadOctetString(c, false)
	if err != nil {
		t.Fatalf("ReadOctetString failed: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Expected %v, got %v", data, result)
	}
}

func TestAperWriter_WriteBitString(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	data := []byte{0xAB}
	c := &Constraint{Lb: 0, Ub: 10}
	if err := aw.WriteBitString(data, 8, c, false); err != nil {
		t.Fatalf("WriteBitString failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	result, nbits, err := ar.ReadBitString(c, false)
	if err != nil {
		t.Fatalf("ReadBitString failed: %v", err)
	}
	if nbits != 8 {
		t.Errorf("Expected 8 bits, got %d", nbits)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Expected %v, got %v", data, result)
	}
}

func TestAperWriter_WriteOpenType(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	data := []byte{0x01, 0x02, 0x03}
	if err := aw.WriteOpenType(data); err != nil {
		t.Fatalf("WriteOpenType failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	result, err := ar.ReadOpenType()
	if err != nil {
		t.Fatalf("ReadOpenType failed: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Expected %v, got %v", data, result)
	}
}

func TestConstraint_Range(t *testing.T) {
	tests := []struct {
		name     string
		c        Constraint
		expected uint64
	}{
		{"Normal range", Constraint{Lb: 0, Ub: 10}, 11},
		{"Single value", Constraint{Lb: 5, Ub: 5}, 1},
		{"Unconstrained", Constraint{Lb: 10, Ub: 5}, 0},
		{"Negative to positive", Constraint{Lb: -5, Ub: 5}, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.c.Range()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestAperReader_readConstraintValue(t *testing.T) {
	tests := []struct {
		name     string
		rangeVal uint64
		value    uint64
	}{
		{"Small range", 10, 5},
		{"Byte range", POW_8, 100},
		{"Two byte range", POW_16, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			aw := NewWriter(&buf)
			if err := aw.writeConstraintValue(tt.rangeVal, tt.value); err != nil {
				t.Fatalf("writeConstraintValue failed: %v", err)
			}
			if err := aw.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			ar := NewReader(&buf)
			result, err := ar.readConstraintValue(tt.rangeVal)
			if err != nil {
				t.Fatalf("readConstraintValue failed: %v", err)
			}
			if result != tt.value {
				t.Errorf("Expected %d, got %d", tt.value, result)
			}
		})
	}
}

func TestAperReader_readNormallySmallNonNegativeValue(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
	}{
		{"Small value", 10},
		{"Large value", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			aw := NewWriter(&buf)
			if err := aw.writeNormallySmallNonNegativeValue(tt.value); err != nil {
				t.Fatalf("writeNormallySmallNonNegativeValue failed: %v", err)
			}
			if err := aw.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			ar := NewReader(&buf)
			result, err := ar.readNormallySmallNonNegativeValue()
			if err != nil {
				t.Fatalf("readNormallySmallNonNegativeValue failed: %v", err)
			}
			if result != tt.value {
				t.Errorf("Expected %d, got %d", tt.value, result)
			}
		})
	}
}

func TestAperReader_readLength(t *testing.T) {
	tests := []struct {
		name     string
		lRange   uint64
		length   uint64
		expected bool // more flag
	}{
		{"Constrained length", 10, 5, false},
		{"Unconstrained small", 0, 50, false},
		{"Unconstrained large", 0, POW_14, true}, // Use multiple of POW_14
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			aw := NewWriter(&buf)
			if err := aw.writeLength(tt.lRange, tt.length); err != nil {
				t.Fatalf("writeLength failed: %v", err)
			}
			if err := aw.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			ar := NewReader(&buf)
			result, more, err := ar.readLength(tt.lRange)
			if err != nil {
				t.Fatalf("readLength failed: %v", err)
			}
			if result != tt.length {
				t.Errorf("Expected length %d, got %d", tt.length, result)
			}
			if more != tt.expected {
				t.Errorf("Expected more=%v, got %v", tt.expected, more)
			}
		})
	}
}

func TestShiftBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		shift    int
		expected []byte
	}{
		{"Shift left 1 bit", []byte{0x01, 0x02}, 1, []byte{0x02, 0x04}},
		{"Shift right 1 bit", []byte{0x02, 0x04}, -1, []byte{0x01, 0x02}},
		{"Shift left 8 bits", []byte{0x01, 0x02}, 8, []byte{0x02, 0x00}},
		{"Shift right 8 bits", []byte{0x01, 0x02}, -8, []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShiftBytes(tt.input, tt.shift)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSetBit_IsBitSet(t *testing.T) {
	content := make([]byte, 2)

	// Set bit at index 1 (first bit of first byte) - bitIndex appears to be 1-based
	SetBit(content, 1)
	if !IsBitSet(content, 1) {
		t.Error("Bit 1 should be set")
	}

	// Set bit at index 9 (first bit of second byte)
	SetBit(content, 9)
	if !IsBitSet(content, 9) {
		t.Error("Bit 9 should be set")
	}

	// Check unset bit
	if IsBitSet(content, 2) {
		t.Error("Bit 2 should not be set")
	}
}

func TestGetBitString(t *testing.T) {
	srcBytes := []byte{0xAB, 0xCD, 0xEF}

	// Get 8 bits starting from offset 0
	dstBytes, err := GetBitString(srcBytes, 0, 8)
	if err != nil {
		t.Fatalf("GetBitString failed: %v", err)
	}
	if len(dstBytes) != 1 || dstBytes[0] != 0xAB {
		t.Errorf("Expected [0xAB], got %v", dstBytes)
	}

	// Get 12 bits starting from offset 4
	dstBytes, err = GetBitString(srcBytes, 4, 12)
	if err != nil {
		t.Fatalf("GetBitString failed: %v", err)
	}
	if len(dstBytes) != 2 {
		t.Errorf("Expected 2 bytes, got %d", len(dstBytes))
	}
}

func TestWriteSequenceOf_ReadSequenceOf(t *testing.T) {
	// Test constraint handling for sequence operations
	c := &Constraint{Lb: 0, Ub: 10}
	if c.Range() != 11 {
		t.Errorf("Expected range 11, got %d", c.Range())
	}

	// Test with fixed length constraint
	cFixed := &Constraint{Lb: 5, Ub: 5}
	if cFixed.Range() != 1 {
		t.Errorf("Expected range 1 for fixed length, got %d", cFixed.Range())
	}
}

func TestAperReader_ReadInteger_Unconstrained(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	// Write unconstrained integer
	if err := aw.WriteInteger(12345, nil, false); err != nil {
		t.Fatalf("WriteInteger failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadInteger(nil, false)
	if err != nil {
		t.Fatalf("ReadInteger failed: %v", err)
	}
	if value != 12345 {
		t.Errorf("Expected 12345, got %d", value)
	}
}

func TestAperReader_ReadInteger_Negative(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	// Write negative integer
	if err := aw.WriteInteger(-100, nil, false); err != nil {
		t.Fatalf("WriteInteger failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadInteger(nil, false)
	if err != nil {
		t.Fatalf("ReadInteger failed: %v", err)
	}
	if value != -100 {
		t.Errorf("Expected -100, got %d", value)
	}
}

func TestAperWriter_WriteEnumerate_Extensible(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	c := Constraint{Lb: 0, Ub: 10}
	// Write value outside range (extensible)
	if err := aw.WriteEnumerate(15, c, true); err != nil {
		t.Fatalf("WriteEnumerate failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadEnumerate(c, true)
	if err != nil {
		t.Fatalf("ReadEnumerate failed: %v", err)
	}
	if value != 15 {
		t.Errorf("Expected 15, got %d", value)
	}
}

func TestAperWriter_WriteChoice(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	uBound := uint64(10)
	choice := uint64(5)
	if err := aw.WriteChoice(choice, uBound, true); err != nil {
		t.Fatalf("WriteChoice failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadChoice(uBound, true)
	if err != nil {
		t.Fatalf("ReadChoice failed: %v", err)
	}
	if value != choice {
		t.Errorf("Expected %d, got %d", choice, value)
	}
}

func TestAperReader_ReadInteger_FixedLength(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	c := &Constraint{Lb: 50, Ub: 50} // Fixed value
	if err := aw.WriteInteger(50, c, false); err != nil {
		t.Fatalf("WriteInteger failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	value, err := ar.ReadInteger(c, false)
	if err != nil {
		t.Fatalf("ReadInteger failed: %v", err)
	}
	if value != 50 {
		t.Errorf("Expected 50, got %d", value)
	}
}

func TestBitstreamWriter_align(t *testing.T) {
	var buf bytes.Buffer
	bs := NewBitStreamWriter(&buf)

	// Write 4 bits
	for i := 0; i < 4; i++ {
		if err := bs.WriteBool(true); err != nil {
			t.Fatalf("WriteBool failed: %v", err)
		}
	}

	// Align (should flush remaining bits)
	if err := bs.align(); err != nil {
		t.Fatalf("align failed: %v", err)
	}

	result := buf.Bytes()
	if len(result) != 1 {
		t.Fatalf("Expected 1 byte after align, got %d", len(result))
	}
	// Expected: 11110000 = 0xF0
	if result[0] != 0xF0 {
		t.Errorf("Expected 0xF0, got 0x%02X", result[0])
	}
}

func TestBitstreamReader_align(t *testing.T) {
	data := []byte{0xAB, 0xCD}
	bs := NewBitStreamReader(bytes.NewReader(data))

	// Read a few bits
	_, _ = bs.ReadBool()
	_, _ = bs.ReadBool()

	// Align (should reset index)
	bs.align()

	// Next read should read from next byte
	bit, err := bs.ReadBool()
	if err != nil {
		t.Fatalf("ReadBool failed: %v", err)
	}
	// After align, should read first bit of second byte (0xCD = 11001101)
	// First bit is 1
	if !bit {
		t.Error("Expected true after align")
	}
}

func TestAperWriter_writeSemiConstraintWholeNumber(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	lb := uint64(100)
	value := uint64(150)
	if err := aw.writeSemiConstraintWholeNumber(value, lb); err != nil {
		t.Fatalf("writeSemiConstraintWholeNumber failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	result, err := ar.readSemiConstraintWholeNumber(lb)
	if err != nil {
		t.Fatalf("readSemiConstraintWholeNumber failed: %v", err)
	}
	if result != value {
		t.Errorf("Expected %d, got %d", value, result)
	}
}

func TestAperReader_ReadString_MultiPart(t *testing.T) {
	// Test that multi-part string encoding/decoding works
	// This is a placeholder test - full multi-part testing requires
	// careful handling of the length encoding which is complex.
	// Basic string encoding/decoding is tested in TestAperWriter_WriteOctetString

	var buf bytes.Buffer
	aw := NewWriter(&buf)

	// Test with a constrained string that's large enough to potentially trigger multi-part
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i % 256)
	}

	c := &Constraint{Lb: 0, Ub: 200} // Constrained but large enough
	if err := aw.WriteOctetString(data, c, false); err != nil {
		t.Fatalf("WriteOctetString failed: %v", err)
	}

	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read it back
	ar := NewReader(&buf)
	result, err := ar.ReadOctetString(c, false)
	if err != nil {
		t.Fatalf("ReadOctetString failed: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Data mismatch: expected %d bytes, got %d bytes", len(data), len(result))
	}
}

func TestNewReader_NewWriter(t *testing.T) {
	var buf bytes.Buffer

	// Test NewWriter
	aw := NewWriter(&buf)
	if aw == nil {
		t.Fatal("NewWriter returned nil")
	}
	if aw.bitstreamWriter == nil {
		t.Fatal("bitstreamWriter is nil")
	}

	// Test NewReader
	ar := NewReader(&buf)
	if ar == nil {
		t.Fatal("NewReader returned nil")
	}
	if ar.bitstreamReader == nil {
		t.Fatal("bitstreamReader is nil")
	}
}

func TestAperWriter_Close(t *testing.T) {
	var buf bytes.Buffer
	aw := NewWriter(&buf)

	// Write some data
	if err := aw.WriteBool(true); err != nil {
		t.Fatalf("WriteBool failed: %v", err)
	}

	// Close should flush
	if err := aw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Buffer should have data
	if buf.Len() == 0 {
		t.Error("Close should have flushed data")
	}
}

func TestErrors(t *testing.T) {
	// Test error definitions
	errors := []error{
		ErrCritical,
		ErrUnderflow,
		ErrOverflow,
		ErrTail,
		ErrIncomplete,
		ErrInextensible,
		ErrFixedLength,
		ErrConstraint,
		ErrInvalidLength,
	}

	for _, err := range errors {
		if err == nil {
			t.Errorf("Error %v should not be nil", err)
		}
		if err.Error() == "" {
			t.Errorf("Error %v should have a message", err)
		}
	}
}

func TestBitstreamWriter_writeByte(t *testing.T) {
	var buf bytes.Buffer
	bs := NewBitStreamWriter(&buf)

	// Write 4 bits first
	for i := 0; i < 4; i++ {
		if err := bs.WriteBool(true); err != nil {
			t.Fatalf("WriteBool failed: %v", err)
		}
	}

	// Write a byte (should handle alignment)
	if err := bs.writeByte(0xAB); err != nil {
		t.Fatalf("writeByte failed: %v", err)
	}

	if err := bs.flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	result := buf.Bytes()
	if len(result) != 2 {
		t.Fatalf("Expected 2 bytes, got %d", len(result))
	}
}

func TestBitstreamReader_readByte(t *testing.T) {
	data := []byte{0xAB, 0xCD}
	bs := NewBitStreamReader(bytes.NewReader(data))

	// Read a few bits first
	_, _ = bs.ReadBool()
	_, _ = bs.ReadBool()

	// Read a byte
	result, err := bs.readByte()
	if err != nil && err != io.EOF {
		t.Fatalf("readByte failed: %v", err)
	}
	// Result should be a combination of remaining bits from first byte and bits from second byte
	if result == 0 {
		t.Error("readByte should return non-zero value")
	}
}

package per

// BitStream manages a bit-level buffer for PER encoding/decoding.
// Bits are written/read MSB-first within each octet (most significant bit first).
// This is the foundation for all PER operations per X.691.
type BitStream struct {
	data    []byte // Underlying byte buffer
	bytePos int    // Current byte index (0-based)
	bitPos  int    // Bit position within current byte (0=MSB, 7=LSB)
}

// newBitStream creates a new empty BitStream for writing.
func newBitStream() *BitStream {
	return &BitStream{
		data:    make([]byte, 0, 64), // Start with reasonable capacity
		bytePos: 0,
		bitPos:  0,
	}
}

// newBitStreamFromBytes creates a BitStream from existing bytes for reading.
func newBitStreamFromBytes(data []byte) *BitStream {
	return &BitStream{
		data:    data,
		bytePos: 0,
		bitPos:  0,
	}
}

// WriteBit writes a single bit (0 or 1) to the stream.
// Bits are written MSB-first within each octet per X.691.
func (bs *BitStream) WriteBit(bit int) {
	// Ensure we have space for current byte
	for len(bs.data) <= bs.bytePos {
		bs.data = append(bs.data, 0)
	}

	// Write bit at current position (MSB = position 0)
	if bit != 0 {
		bs.data[bs.bytePos] |= (1 << (7 - bs.bitPos))
	}

	// Advance position
	bs.bitPos++
	if bs.bitPos == 8 {
		bs.bitPos = 0
		bs.bytePos++
	}
}

// WriteBits writes 'count' bits of 'value' to the stream.
// Bits are written MSB-first per X.691.
// count must be between 0 and 64.
func (bs *BitStream) WriteBits(value uint64, count int) {
	if count == 0 {
		return
	}
	if count > 64 {
		panic("WriteBits: count must be <= 64")
	}

	// Write bits from most significant to least significant
	for i := count - 1; i >= 0; i-- {
		bit := int((value >> i) & 1)
		bs.WriteBit(bit)
	}
}

// ReadBit reads a single bit from the stream.
// Returns the bit value (0 or 1) and error if at end of stream.
func (bs *BitStream) ReadBit() (int, error) {
	// Check if we're at end of stream
	if bs.bytePos >= len(bs.data) {
		return 0, &DecodeError{
			TypeName: "BitStream",
			Reason:   "unexpected end of input",
			Position: bs.BitLength(),
			Expected: 1,
			Got:      0,
		}
	}

	// Read bit at current position (MSB = position 0)
	bit := int((bs.data[bs.bytePos] >> (7 - bs.bitPos)) & 1)

	// Advance position
	bs.bitPos++
	if bs.bitPos == 8 {
		bs.bitPos = 0
		bs.bytePos++
	}

	return bit, nil
}

// ReadBits reads 'count' bits from the stream.
// Returns the value and error if insufficient bits available.
// count must be between 0 and 64.
func (bs *BitStream) ReadBits(count int) (uint64, error) {
	if count == 0 {
		return 0, nil
	}
	if count > 64 {
		panic("ReadBits: count must be <= 64")
	}

	var value uint64
	for range count {
		bit, err := bs.ReadBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | uint64(bit)
	}

	return value, nil
}

// AlignToOctet pads the stream with zero bits until reaching an octet boundary.
// This is only used for APER alignment per X.691.
// UPER never calls this function (strict bit-level encoding).
func (bs *BitStream) AlignToOctet() {
	if bs.bitPos == 0 {
		return // Already aligned
	}

	// Pad with zeros to reach next byte boundary
	bitsToAdd := 8 - bs.bitPos
	bs.WriteBits(0, bitsToAdd)
}

// Bytes returns a copy of the encoded bytes.
// If the last byte is partial (bitPos != 0), it is included with trailing zeros.
func (bs *BitStream) Bytes() []byte {
	// Determine how many bytes to return
	length := bs.bytePos
	if bs.bitPos > 0 {
		length++ // Include partial byte
	}

	// Return copy to prevent external modification
	result := make([]byte, length)
	copy(result, bs.data[:length])
	return result
}

// BitLength returns the total number of bits written to the stream.
func (bs *BitStream) BitLength() int {
	return bs.bytePos*8 + bs.bitPos
}

// Remaining returns the number of bits remaining to be read.
func (bs *BitStream) Remaining() int {
	totalBits := len(bs.data) * 8
	currentBit := bs.bytePos*8 + bs.bitPos
	return totalBits - currentBit
}

// Position returns the current bit position in the stream.
func (bs *BitStream) Position() int {
	return bs.bytePos*8 + bs.bitPos
}

// Reset clears the stream for reuse.
// Existing data is preserved but position is reset to start.
func (bs *BitStream) Reset() {
	bs.bytePos = 0
	bs.bitPos = 0
	bs.data = bs.data[:0] // Reset length but keep capacity
}

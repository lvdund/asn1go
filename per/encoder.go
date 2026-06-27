package per

// Encoder encodes ASN.1 values to PER format.
// Create an encoder with NewEncoder() specifying the variant (APER or UPER).
// The encoder maintains a bit-level buffer and tracks the current encoding position.
type Encoder struct {
	variant Variant
	stream  *BitStream
}

// NewEncoder creates a new PER encoder with the specified variant.
// The variant (APER or UPER) affects alignment behavior throughout encoding.
// Per constitution: variant cannot change during encoding operation.
func NewEncoder(variant Variant) *Encoder {
	return &Encoder{
		variant: variant,
		stream:  newBitStream(),
	}
}

// Bytes returns a copy of the encoded bytes.
// The returned slice is safe to modify (it's a copy, not a reference).
// If the encoding ends mid-byte, the partial byte is included with trailing zeros.
func (e *Encoder) Bytes() []byte {
	return e.stream.Bytes()
}

// BitLength returns the total number of bits written.
// This is useful for debugging and verifying encodings match the specification.
func (e *Encoder) BitLength() int {
	return e.stream.BitLength()
}

// Reset clears the encoder's buffer for reuse.
// The variant remains unchanged (per constitution).
// This is more efficient than creating a new encoder for repeated encodings.
func (e *Encoder) Reset() {
	e.stream.Reset()
}

// Variant returns the encoder's variant (APER or UPER).
func (e *Encoder) Variant() Variant {
	return e.variant
}

package per

// Decoder decodes PER format to ASN.1 values.
// Create a decoder with NewDecoder() providing the encoded bytes and variant.
// The decoder maintains position state and validates sufficient data is available.
type Decoder struct {
	variant Variant
	stream  *BitStream
}

// NewDecoder creates a new PER decoder with the specified data and variant.
// The data slice is not copied, so it must not be modified during decoding.
// The variant (APER or UPER) must match the variant used during encoding.
func NewDecoder(data []byte, variant Variant) *Decoder {
	return &Decoder{
		variant: variant,
		stream:  newBitStreamFromBytes(data),
	}
}

// Remaining returns the number of bits remaining to be read.
// This is useful for validating complete messages and detecting truncation.
func (d *Decoder) Remaining() int {
	return d.stream.Remaining()
}

// Position returns the current bit position in the stream.
// Position 0 is the first bit. Useful for error reporting per constitution.
func (d *Decoder) Position() int {
	return d.stream.Position()
}

// Variant returns the decoder's variant (APER or UPER).
func (d *Decoder) Variant() Variant {
	return d.variant
}

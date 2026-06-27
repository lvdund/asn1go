package per

// BitString represents an ASN.1 BIT STRING value.
// Bits are stored with MSB first (network byte order).
type BitString struct {
	Bits   []byte // Bit data, packed into bytes
	Length int    // Number of valid bits
}

// NewBitString creates a BitString from a byte slice and bit count.
func NewBitString(bits []byte, length int) BitString {
	return BitString{
		Bits:   bits,
		Length: length,
	}
}

// BitAt returns the bit value at the given index (0-based, MSB first).
func (bs BitString) BitAt(index int) uint8 {
	if index < 0 || index >= bs.Length {
		return 0
	}
	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	return (bs.Bits[byteIndex] >> bitIndex) & 1
}

// EncodeBitString encodes a BIT STRING value per X.691 Clauses 16-17.
// Encoding varies based on size constraints:
// - Fixed small (SIZE = n, n ≤ 16): direct bit encoding
// - Fixed large (SIZE = n, n > 16): APER alignment + bits
// - Variable: length determinant + APER alignment + bits
func (e *Encoder) EncodeBitString(value BitString, constraints SizeConstraints) error {
	n := int64(value.Length)

	// X.691 Clause 16.3: Check for extensibility
	if constraints.Extensible {
		inRoot := false
		if constraints.Min != nil && constraints.Max != nil {
			inRoot = n >= *constraints.Min && n <= *constraints.Max
		}

		if inRoot {
			// Extension bit: 0 = in root
			e.stream.WriteBit(0)
			return e.encodeRootBitString(value, constraints)
		} else {
			// Extension bit: 1 = in extension
			e.stream.WriteBit(1)
			return e.encodeExtensionBitString(value)
		}
	}

	// No extension marker
	return e.encodeRootBitString(value, constraints)
}

// encodeRootBitString handles root BIT STRING encoding.
func (e *Encoder) encodeRootBitString(value BitString, constraints SizeConstraints) error {
	n := int64(value.Length)
	min := constraints.Min
	max := constraints.Max

	// X.691 Clause 16.5: Zero length
	if max != nil && min != nil && *max == *min && *max == 0 {
		// No bits emitted
		return nil
	}

	// X.691 Clause 16.6: Fixed size, small (≤ 16 bits)
	if max != nil && min != nil && *max == *min && *max <= 16 {
		if n != *max {
			return &ConstraintViolationError{
				TypeName:   "BIT STRING",
				Value:      n,
				Constraint: "length must match fixed size",
				Position:   e.stream.Position(),
			}
		}
		// Write bits directly, no alignment
		for i := 0; i < value.Length; i++ {
			e.stream.WriteBit(int(value.BitAt(i)))
		}
		return nil
	}

	// X.691 Clause 16.7: Fixed size, large (> 16 bits)
	if max != nil && min != nil && *max == *min && *max > 16 {
		if n != *max {
			return &ConstraintViolationError{
				TypeName:   "BIT STRING",
				Value:      n,
				Constraint: "length must match fixed size",
				Position:   e.stream.Position(),
			}
		}
		// APER: Align to octet boundary
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		// Write bits
		for i := 0; i < value.Length; i++ {
			e.stream.WriteBit(int(value.BitAt(i)))
		}
		return nil
	}

	// X.691 Clause 16.8-16.9: Variable length
	// Encode length determinant
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return err
	}

	// APER: Align to octet boundary before value
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Write bits
	for i := 0; i < value.Length; i++ {
		e.stream.WriteBit(int(value.BitAt(i)))
	}

	return nil
}

// encodeExtensionBitString handles extension BIT STRING encoding.
func (e *Encoder) encodeExtensionBitString(value BitString) error {
	n := int64(value.Length)

	// Length is unconstrained in extension
	constraints := SizeConstraints{Min: nil, Max: nil}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return err
	}

	// APER: Align to octet boundary
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Write bits
	for i := 0; i < value.Length; i++ {
		e.stream.WriteBit(int(value.BitAt(i)))
	}

	return nil
}

// DecodeBitString decodes a BIT STRING value per X.691 Clauses 16-17.
func (d *Decoder) DecodeBitString(constraints SizeConstraints) (BitString, error) {
	// X.691 Clause 16.3: Check for extensibility
	if constraints.Extensible {
		// Read extension bit
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return BitString{}, &DecodeError{
				TypeName: "BIT STRING",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}

		if extBit == 1 {
			// In extension: decode with unconstrained length
			return d.decodeExtensionBitString()
		}
	}

	// In root
	return d.decodeRootBitString(constraints)
}

// decodeRootBitString handles root BIT STRING decoding.
func (d *Decoder) decodeRootBitString(constraints SizeConstraints) (BitString, error) {
	min := constraints.Min
	max := constraints.Max

	// Zero length
	if max != nil && min != nil && *max == *min && *max == 0 {
		return BitString{Bits: []byte{}, Length: 0}, nil
	}

	// Fixed size, small (≤ 16 bits)
	if max != nil && min != nil && *max == *min && *max <= 16 {
		length := int(*max)
		bits, err := d.readBits(length)
		if err != nil {
			return BitString{}, err
		}
		return bits, nil
	}

	// Fixed size, large (> 16 bits)
	if max != nil && min != nil && *max == *min && *max > 16 {
		// APER: Read alignment padding
		if d.variant == APER {
			for d.stream.Position()%8 != 0 {
				_, err := d.stream.ReadBit()
				if err != nil {
					return BitString{}, err
				}
			}
		}

		length := int(*max)
		bits, err := d.readBits(length)
		if err != nil {
			return BitString{}, err
		}
		return bits, nil
	}

	// Variable length: decode length determinant
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return BitString{}, err
	}

	// APER: Read alignment padding
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return BitString{}, err
			}
		}
	}

	bits, err := d.readBits(int(length))
	if err != nil {
		return BitString{}, err
	}
	return bits, nil
}

// decodeExtensionBitString handles extension BIT STRING decoding.
func (d *Decoder) decodeExtensionBitString() (BitString, error) {
	// Unconstrained length
	constraints := SizeConstraints{Min: nil, Max: nil}
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return BitString{}, err
	}

	// APER: Read alignment padding
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return BitString{}, err
			}
		}
	}

	bits, err := d.readBits(int(length))
	if err != nil {
		return BitString{}, err
	}
	return bits, nil
}

// readBits reads n bits from the stream and returns as BitString.
func (d *Decoder) readBits(n int) (BitString, error) {
	if n == 0 {
		return BitString{Bits: []byte{}, Length: 0}, nil
	}

	// Calculate bytes needed
	numBytes := (n + 7) / 8
	bits := make([]byte, numBytes)

	// Read bits and pack into bytes
	for i := range n {
		bit, err := d.stream.ReadBit()
		if err != nil {
			return BitString{}, &DecodeError{
				TypeName: "BIT STRING",
				Reason:   "failed to read bit",
				Position: d.stream.Position(),
			}
		}

		if bit != 0 {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			bits[byteIndex] |= (1 << bitIndex)
		}
	}

	return BitString{Bits: bits, Length: n}, nil
}

package per

// EncodeOctetString encodes an OCTET STRING value per X.691 Clause 17.
// Encoding varies based on size constraints:
// - Fixed small (SIZE = n, n ≤ 2): direct octet encoding, no alignment
// - Fixed large (SIZE = n, n > 2): APER alignment + octets
// - Variable: length determinant + APER alignment + octets
func (e *Encoder) EncodeOctetString(value []byte, constraints SizeConstraints) error {
	n := int64(len(value))

	// X.691 Clause 17.3: Check for extensibility
	if constraints.Extensible {
		inRoot := false
		if constraints.Min != nil && constraints.Max != nil {
			inRoot = n >= *constraints.Min && n <= *constraints.Max
		}

		if inRoot {
			// Extension bit: 0 = in root
			e.stream.WriteBit(0)
			return e.encodeRootOctetString(value, constraints)
		} else {
			// Extension bit: 1 = in extension
			e.stream.WriteBit(1)
			return e.encodeExtensionOctetString(value)
		}
	}

	// No extension marker
	return e.encodeRootOctetString(value, constraints)
}

// encodeRootOctetString handles root OCTET STRING encoding.
func (e *Encoder) encodeRootOctetString(value []byte, constraints SizeConstraints) error {
	n := int64(len(value))
	min := constraints.Min
	max := constraints.Max

	// X.691 Clause 17.4: Zero length
	if max != nil && *max == 0 {
		if n != 0 {
			return &ConstraintViolationError{
				TypeName:   "OCTET STRING",
				Value:      n,
				Constraint: "length must be zero",
				Position:   e.stream.Position(),
			}
		}
		// No octets emitted
		return nil
	}

	// X.691 Clause 17.5: Fixed size, small (≤ 2 octets)
	if max != nil && min != nil && *max == *min && *max <= 2 {
		if n != *max {
			return &ConstraintViolationError{
				TypeName:   "OCTET STRING",
				Value:      n,
				Constraint: "length must match fixed size",
				Position:   e.stream.Position(),
			}
		}
		// Write octets directly, no alignment
		for _, octet := range value {
			e.stream.WriteBits(uint64(octet), 8)
		}
		return nil
	}

	// X.691 Clause 17.6: Fixed size, large (> 2 octets)
	if max != nil && min != nil && *max == *min && *max > 2 {
		if n != *max {
			return &ConstraintViolationError{
				TypeName:   "OCTET STRING",
				Value:      n,
				Constraint: "length must match fixed size",
				Position:   e.stream.Position(),
			}
		}
		// APER: Align to octet boundary
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		// Write octets
		for _, octet := range value {
			e.stream.WriteBits(uint64(octet), 8)
		}
		return nil
	}

	// X.691 Clause 17.7-17.8: Variable length
	// Encode length determinant
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return err
	}

	// APER: Align to octet boundary before value
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Write octets
	for _, octet := range value {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeExtensionOctetString handles extension OCTET STRING encoding.
func (e *Encoder) encodeExtensionOctetString(value []byte) error {
	n := int64(len(value))

	// Length is unconstrained in extension
	constraints := SizeConstraints{Min: nil, Max: nil}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return err
	}

	// APER: Align to octet boundary
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Write octets
	for _, octet := range value {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// DecodeOctetString decodes an OCTET STRING value per X.691 Clause 17.
func (d *Decoder) DecodeOctetString(constraints SizeConstraints) ([]byte, error) {
	// X.691 Clause 17.3: Check for extensibility
	if constraints.Extensible {
		// Read extension bit
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return nil, &DecodeError{
				TypeName: "OCTET STRING",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}

		if extBit == 1 {
			// In extension: decode with unconstrained length
			return d.decodeExtensionOctetString()
		}
	}

	// In root
	return d.decodeRootOctetString(constraints)
}

// decodeRootOctetString handles root OCTET STRING decoding.
func (d *Decoder) decodeRootOctetString(constraints SizeConstraints) ([]byte, error) {
	min := constraints.Min
	max := constraints.Max

	// Zero length
	if max != nil && *max == 0 {
		return []byte{}, nil
	}

	// Fixed size, small (≤ 2 octets)
	if max != nil && min != nil && *max == *min && *max <= 2 {
		length := int(*max)
		return d.readOctetsUnaligned(length)
	}

	// Fixed size, large (> 2 octets)
	if max != nil && min != nil && *max == *min && *max > 2 {
		// APER: Read alignment padding
		if d.variant == APER {
			for d.stream.Position()%8 != 0 {
				_, err := d.stream.ReadBit()
				if err != nil {
					return nil, err
				}
			}
		}

		length := int(*max)
		return d.readOctetsAligned(length)
	}

	// Variable length: decode length determinant
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return nil, err
	}

	// APER: Read alignment padding
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return nil, err
			}
		}
	}

	return d.readOctetsAligned(int(length))
}

// decodeExtensionOctetString handles extension OCTET STRING decoding.
func (d *Decoder) decodeExtensionOctetString() ([]byte, error) {
	// Unconstrained length
	constraints := SizeConstraints{Min: nil, Max: nil}
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return nil, err
	}

	// APER: Read alignment padding
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return nil, err
			}
		}
	}

	return d.readOctetsAligned(int(length))
}

// readOctetsUnaligned reads n octets without assuming octet alignment.
func (d *Decoder) readOctetsUnaligned(n int) ([]byte, error) {
	octets := make([]byte, n)
	for i := range n {
		octet, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "OCTET STRING",
				Reason:   "failed to read octet",
				Position: d.stream.Position(),
			}
		}
		octets[i] = byte(octet)
	}
	return octets, nil
}

// readOctetsAligned reads n octets (assumes already aligned for APER).
func (d *Decoder) readOctetsAligned(n int) ([]byte, error) {
	octets := make([]byte, n)
	for i := range n {
		octet, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "OCTET STRING",
				Reason:   "failed to read octet",
				Position: d.stream.Position(),
			}
		}
		octets[i] = byte(octet)
	}
	return octets, nil
}

package per

// EncodeEnumerated encodes an ENUMERATED value per X.691 Clause 14.
// The value is encoded as an index into the enumeration:
// - Root values: constrained integer (0..count-1)
// - Extension values: normally small non-negative number
func (e *Encoder) EncodeEnumerated(index int64, constraints EnumeratedConstraints) error {
	// X.691 Clause 14.3: Check for extensibility
	rootCount := int64(len(constraints.RootValues))

	if constraints.Extensible {
		// Determine if index is in root or extension
		if index < rootCount {
			// Extension bit: 0 = in root
			e.stream.WriteBit(0)
			return e.encodeRootEnumerated(index, rootCount)
		} else {
			// Extension bit: 1 = in extension
			e.stream.WriteBit(1)
			// Extension index is offset from root count
			extIndex := index - rootCount
			return e.encodeNormallySmallNumber(extIndex)
		}
	}

	// No extension marker - encode as root
	return e.encodeRootEnumerated(index, rootCount)
}

// encodeRootEnumerated encodes a root ENUMERATED value as a constrained integer.
func (e *Encoder) encodeRootEnumerated(index, rootCount int64) error {
	if index < 0 || index >= rootCount {
		return &ConstraintViolationError{
			TypeName:   "ENUMERATED",
			Value:      index,
			Constraint: "index out of root range",
			Position:   e.stream.Position(),
		}
	}

	// Single value in root
	if rootCount == 1 {
		// No bits emitted
		return nil
	}

	// Encode as constrained integer (0..rootCount-1)
	constraints := Constrained(0, rootCount-1)
	return e.EncodeInteger(index, constraints)
}

// encodeNormallySmallNumber encodes a normally small non-negative whole number per X.691 Clause 11.6.
// This is used for extension indices and other "normally small" values.
func (e *Encoder) encodeNormallySmallNumber(n int64) error {
	if n < 0 {
		return &ConstraintViolationError{
			TypeName:   "ENUMERATED",
			Value:      n,
			Constraint: "normally small number must be non-negative",
			Position:   e.stream.Position(),
		}
	}

	// X.691 Clause 11.6: Normally small number
	if n <= 63 {
		// Small value: bit 0 + 6 bits
		e.stream.WriteBit(0)
		e.stream.WriteBits(uint64(n), 6)
		return nil
	}

	// Large value: bit 1 + semi-constrained integer (lb=0)
	e.stream.WriteBit(1)
	constraints := SemiConstrained(0)
	return e.EncodeInteger(n, constraints)
}

// DecodeEnumerated decodes an ENUMERATED value per X.691 Clause 14.
func (d *Decoder) DecodeEnumerated(constraints EnumeratedConstraints) (int64, error) {
	rootCount := int64(len(constraints.RootValues))

	// X.691 Clause 14.3: Check for extensibility
	if constraints.Extensible {
		// Read extension bit
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return 0, &DecodeError{
				TypeName: "ENUMERATED",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}

		if extBit == 1 {
			// In extension: decode normally small number
			extIndex, err := d.decodeNormallySmallNumber()
			if err != nil {
				return 0, err
			}
			// Return absolute index (root count + extension index)
			return rootCount + extIndex, nil
		}
	}

	// In root: decode as constrained integer
	return d.decodeRootEnumerated(rootCount)
}

// decodeRootEnumerated decodes a root ENUMERATED value.
func (d *Decoder) decodeRootEnumerated(rootCount int64) (int64, error) {
	if rootCount <= 0 {
		return 0, &DecodeError{
			TypeName: "ENUMERATED",
			Reason:   "invalid root count",
			Position: d.stream.Position(),
		}
	}

	// Single value in root
	if rootCount == 1 {
		return 0, nil
	}

	// Decode as constrained integer (0..rootCount-1)
	constraints := Constrained(0, rootCount-1)
	index, err := d.DecodeInteger(constraints)
	if err != nil {
		return 0, &DecodeError{
			TypeName: "ENUMERATED",
			Reason:   "failed to decode root index",
			Position: d.stream.Position(),
		}
	}

	return index, nil
}

// decodeNormallySmallNumber decodes a normally small non-negative whole number per X.691 Clause 11.6.
func (d *Decoder) decodeNormallySmallNumber() (int64, error) {
	// Read flag bit
	flagBit, err := d.stream.ReadBit()
	if err != nil {
		return 0, &DecodeError{
			TypeName: "ENUMERATED",
			Reason:   "failed to read normally small number flag",
			Position: d.stream.Position(),
		}
	}

	if flagBit == 0 {
		// Small value: read 6 bits
		value, err := d.stream.ReadBits(6)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "ENUMERATED",
				Reason:   "failed to read normally small number value",
				Position: d.stream.Position(),
			}
		}
		return int64(value), nil
	}

	// Large value: decode as semi-constrained integer (lb=0)
	constraints := SemiConstrained(0)
	value, err := d.DecodeInteger(constraints)
	if err != nil {
		return 0, &DecodeError{
			TypeName: "ENUMERATED",
			Reason:   "failed to decode large normally small number",
			Position: d.stream.Position(),
		}
	}

	return value, nil
}

package per

// SequenceOfEncoder provides methods for encoding SEQUENCE OF structures.
// SEQUENCE OF encoding per X.691 Clause 20:
// 1. Extension bit (if extensible)
// 2. Length determinant (based on SIZE constraints)
// 3. Component values (repeated)
type SequenceOfEncoder struct {
	encoder     *Encoder
	constraints SizeConstraints
}

// SequenceOfDecoder provides methods for decoding SEQUENCE OF structures.
type SequenceOfDecoder struct {
	decoder     *Decoder
	constraints SizeConstraints
	length      int64 // Decoded length
}

// NewSequenceOfEncoder creates a new SEQUENCE OF encoder.
func (e *Encoder) NewSequenceOfEncoder(constraints SizeConstraints) *SequenceOfEncoder {
	return &SequenceOfEncoder{
		encoder:     e,
		constraints: constraints,
	}
}

// NewSequenceOfDecoder creates a new SEQUENCE OF decoder.
func (d *Decoder) NewSequenceOfDecoder(constraints SizeConstraints) *SequenceOfDecoder {
	return &SequenceOfDecoder{
		decoder:     d,
		constraints: constraints,
	}
}

// EncodeLength encodes the length determinant for SEQUENCE OF.
// The length encoding depends on SIZE constraints.
func (soe *SequenceOfEncoder) EncodeLength(length int64) error {
	// X.691 Clause 20.3: Check extensibility
	if soe.constraints.Extensible {
		inRoot := false
		if soe.constraints.Min != nil && soe.constraints.Max != nil {
			inRoot = length >= *soe.constraints.Min && length <= *soe.constraints.Max
		}

		if inRoot {
			// Extension bit: 0 = in root
			soe.encoder.stream.WriteBit(0)
		} else {
			// Extension bit: 1 = in extension
			soe.encoder.stream.WriteBit(1)
			// Encode length as unconstrained
			unconstrained := SizeConstraints{Min: nil, Max: nil}
			return soe.encoder.EncodeLengthDeterminant(length, unconstrained)
		}
	}

	// X.691 Clause 20.4: Fixed length
	min := soe.constraints.Min
	max := soe.constraints.Max
	if min != nil && max != nil && *min == *max {
		// Fixed length: validate but don't encode
		if length != *min {
			return &ConstraintViolationError{
				TypeName:   "SEQUENCE OF",
				Value:      length,
				Constraint: "length must match fixed size",
				Position:   soe.encoder.stream.Position(),
			}
		}
		return nil
	}

	// X.691 Clause 20.5-20.6: Variable length
	return soe.encoder.EncodeLengthDeterminant(length, soe.constraints)
}

// DecodeLength decodes the length determinant for SEQUENCE OF.
func (sod *SequenceOfDecoder) DecodeLength() (int64, error) {
	// X.691 Clause 20.3: Check extensibility
	if sod.constraints.Extensible {
		extBit, err := sod.decoder.stream.ReadBit()
		if err != nil {
			return 0, &DecodeError{
				TypeName: "SEQUENCE OF",
				Reason:   "failed to read extension bit",
				Position: sod.decoder.stream.Position(),
			}
		}

		if extBit == 1 {
			// In extension: unconstrained length
			unconstrained := SizeConstraints{Min: nil, Max: nil}
			length, err := sod.decoder.DecodeLengthDeterminant(unconstrained)
			if err != nil {
				return 0, err
			}
			sod.length = length
			return length, nil
		}
	}

	// X.691 Clause 20.4: Fixed length
	min := sod.constraints.Min
	max := sod.constraints.Max
	if min != nil && max != nil && *min == *max {
		// Fixed length: no length determinant
		sod.length = *min
		return *min, nil
	}

	// X.691 Clause 20.5-20.6: Variable length
	length, err := sod.decoder.DecodeLengthDeterminant(sod.constraints)
	if err != nil {
		return 0, err
	}

	sod.length = length
	return length, nil
}

// SetOfEncoder is identical to SequenceOfEncoder (SET OF has same encoding).
// The only difference is semantic: SET OF elements should be unique and ordered,
// but PER encoding is identical to SEQUENCE OF.
type SetOfEncoder = SequenceOfEncoder

// SetOfDecoder is identical to SequenceOfDecoder.
type SetOfDecoder = SequenceOfDecoder

// NewSetOfEncoder creates a new SET OF encoder (alias for SequenceOfEncoder).
func (e *Encoder) NewSetOfEncoder(constraints SizeConstraints) *SetOfEncoder {
	return e.NewSequenceOfEncoder(constraints)
}

// NewSetOfDecoder creates a new SET OF decoder (alias for SequenceOfDecoder).
func (d *Decoder) NewSetOfDecoder(constraints SizeConstraints) *SetOfDecoder {
	return d.NewSequenceOfDecoder(constraints)
}

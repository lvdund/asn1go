package per

// ChoiceEncoder provides methods for encoding CHOICE values.
// CHOICE encoding per X.691 Clause 23 involves:
// 1. Extension bit (if extensible)
// 2. Alternative index (constrained integer for root, normally small for extension)
// 3. Alternative value (direct for root, as open type for extension)
type ChoiceEncoder struct {
	encoder     *Encoder
	constraints ChoiceConstraints
}

// ChoiceDecoder provides methods for decoding CHOICE values.
type ChoiceDecoder struct {
	decoder     *Decoder
	constraints ChoiceConstraints
	isExtension bool  // Whether chosen alternative is in extension
	chosenIndex int64 // Index of chosen alternative
}

// NewChoiceEncoder creates a new CHOICE encoder.
func (e *Encoder) NewChoiceEncoder(constraints ChoiceConstraints) *ChoiceEncoder {
	return &ChoiceEncoder{
		encoder:     e,
		constraints: constraints,
	}
}

// NewChoiceDecoder creates a new CHOICE decoder.
func (d *Decoder) NewChoiceDecoder(constraints ChoiceConstraints) *ChoiceDecoder {
	return &ChoiceDecoder{
		decoder:     d,
		constraints: constraints,
	}
}

// EncodeChoice encodes a CHOICE value given the alternative index.
// For root alternatives: index is 0-based in canonical order.
// For extension alternatives: index is 0-based in definition order.
func (ce *ChoiceEncoder) EncodeChoice(alternativeIndex int64, isExtension bool, value []byte) error {
	// X.691 Clause 23.3: Extension bit
	if ce.constraints.Extensible {
		if isExtension {
			ce.encoder.stream.WriteBit(1)
		} else {
			ce.encoder.stream.WriteBit(0)
		}
	}

	if isExtension {
		// Extension alternative: encode as open type
		return ce.encodeExtensionAlternative(alternativeIndex, value)
	}

	// Root alternative
	return ce.encodeRootAlternative(alternativeIndex, value)
}

// encodeRootAlternative encodes a root CHOICE alternative.
func (ce *ChoiceEncoder) encodeRootAlternative(index int64, value []byte) error {
	numRoot := int64(len(ce.constraints.RootAlternatives))

	// Validate index
	if index < 0 || index >= numRoot {
		return &ConstraintViolationError{
			TypeName:   "CHOICE",
			Value:      index,
			Constraint: "alternative index out of range",
			Position:   ce.encoder.stream.Position(),
		}
	}

	// X.691 Clause 23.4: Encode index as constrained integer (0..n-1)
	if numRoot > 1 {
		constraints := Constrained(0, numRoot-1)
		if err := ce.encoder.EncodeInteger(index, constraints); err != nil {
			return err
		}
	}

	// Note: The actual value encoding is left to the caller
	// This just handles the CHOICE framing
	return nil
}

// encodeExtensionAlternative encodes an extension CHOICE alternative as an open
// type per X.691 §23.8 ("encoded as if it were the value of an open type field
// as specified in 11.2").
func (ce *ChoiceEncoder) encodeExtensionAlternative(index int64, value []byte) error {
	// Validate index
	numExt := int64(len(ce.constraints.ExtAlternatives))
	if index < 0 || index >= numExt {
		return &ConstraintViolationError{
			TypeName:   "CHOICE",
			Value:      index,
			Constraint: "extension alternative index out of range",
			Position:   ce.encoder.stream.Position(),
		}
	}

	// X.691 §23.8: index encoded as normally small number.
	if err := ce.encoder.encodeNormallySmallNumber(index); err != nil {
		return err
	}

	// X.691 §23.8: value encoded as an open type (§11.2).
	return ce.encoder.EncodeOpenType(value)
}

// DecodeChoice decodes a CHOICE value, returning the alternative index and whether it's an extension.
// For root alternatives: returns (index, false, nil)
// For extension alternatives: returns (index, true, value_bytes)
func (cd *ChoiceDecoder) DecodeChoice() (int64, bool, []byte, error) {
	// X.691 Clause 23.3: Extension bit
	if cd.constraints.Extensible {
		extBit, err := cd.decoder.stream.ReadBit()
		if err != nil {
			return 0, false, nil, &DecodeError{
				TypeName: "CHOICE",
				Reason:   "failed to read extension bit",
				Position: cd.decoder.stream.Position(),
			}
		}
		cd.isExtension = (extBit == 1)
	}

	if cd.isExtension {
		// Extension alternative: decode as open type
		return cd.decodeExtensionAlternative()
	}

	// Root alternative
	return cd.decodeRootAlternative()
}

// decodeRootAlternative decodes a root CHOICE alternative.
func (cd *ChoiceDecoder) decodeRootAlternative() (int64, bool, []byte, error) {
	numRoot := int64(len(cd.constraints.RootAlternatives))

	if numRoot == 0 {
		return 0, false, nil, &DecodeError{
			TypeName: "CHOICE",
			Reason:   "no root alternatives defined",
			Position: cd.decoder.stream.Position(),
		}
	}

	// Single alternative
	if numRoot == 1 {
		cd.chosenIndex = 0
		return 0, false, nil, nil
	}

	// Multiple alternatives: decode index
	constraints := Constrained(0, numRoot-1)
	index, err := cd.decoder.DecodeInteger(constraints)
	if err != nil {
		return 0, false, nil, &DecodeError{
			TypeName: "CHOICE",
			Reason:   "failed to decode alternative index",
			Position: cd.decoder.stream.Position(),
		}
	}

	cd.chosenIndex = index
	return index, false, nil, nil
}

// decodeExtensionAlternative decodes an extension CHOICE alternative as an open
// type per X.691 §23.8 ("encoded as if it were the value of an open type field
// as specified in 11.2").
func (cd *ChoiceDecoder) decodeExtensionAlternative() (int64, bool, []byte, error) {
	// X.691 §23.8: index encoded as normally small number.
	index, err := cd.decoder.decodeNormallySmallNumber()
	if err != nil {
		return 0, false, nil, &DecodeError{
			TypeName: "CHOICE",
			Reason:   "failed to decode extension alternative index",
			Position: cd.decoder.stream.Position(),
		}
	}

	// Validate index
	numExt := int64(len(cd.constraints.ExtAlternatives))
	if index < 0 || index >= numExt {
		return 0, false, nil, &DecodeError{
			TypeName: "CHOICE",
			Reason:   "extension alternative index out of range",
			Position: cd.decoder.stream.Position(),
		}
	}

	// X.691 §23.8: value decoded as an open type (§11.2).
	value, err := cd.decoder.DecodeOpenType()
	if err != nil {
		return 0, false, nil, err
	}

	cd.chosenIndex = index
	return index, true, value, nil
}

package per

// SequenceEncoder provides methods for encoding SEQUENCE preambles and components.
// SEQUENCE encoding per X.691 Clause 19 involves:
// 1. Extension bit (if extensible)
// 2. Preamble bitmap for optional/default root components
// 3. Root component values
// 4. Extension additions (as open types with length determinants)
type SequenceEncoder struct {
	encoder        *Encoder
	constraints    SequenceConstraints
	optionalBitmap []bool // Presence bitmap for optional/default fields
}

// SequenceDecoder provides methods for decoding SEQUENCE preambles and components.
type SequenceDecoder struct {
	decoder         *Decoder
	constraints     SequenceConstraints
	hasExtensions   bool   // Extension bit value
	optionalBitmap  []bool // Presence bitmap for optional/default fields
	extensionCount  int64  // Number of extension additions present
	extensionBitmap []bool // Presence bitmap for extension additions
}

// NewSequenceEncoder creates a new SEQUENCE encoder.
func (e *Encoder) NewSequenceEncoder(constraints SequenceConstraints) *SequenceEncoder {
	return &SequenceEncoder{
		encoder:        e,
		constraints:    constraints,
		optionalBitmap: make([]bool, 0),
	}
}

// NewSequenceDecoder creates a new SEQUENCE decoder.
func (d *Decoder) NewSequenceDecoder(constraints SequenceConstraints) *SequenceDecoder {
	return &SequenceDecoder{
		decoder:         d,
		constraints:     constraints,
		optionalBitmap:  make([]bool, 0),
		extensionBitmap: make([]bool, 0),
	}
}

// EncodeExtensionBit encodes the extension presence bit (if extensible).
// This should be called first if the SEQUENCE has an extension marker.
func (se *SequenceEncoder) EncodeExtensionBit(hasExtensions bool) error {
	if !se.constraints.Extensible {
		return nil
	}

	if hasExtensions {
		se.encoder.stream.WriteBit(1)
	} else {
		se.encoder.stream.WriteBit(0)
	}
	return nil
}

// EncodePreamble encodes the preamble bitmap for optional/default root components.
// The bitmap indicates presence (1) or absence (0) of each optional/default field.
func (se *SequenceEncoder) EncodePreamble(presenceBitmap []bool) error {
	// Count optional/default fields in root
	optionalCount := 0
	for _, comp := range se.constraints.RootComponents {
		if comp.Optional || comp.HasDefault {
			optionalCount++
		}
	}

	// Validate bitmap size
	if len(presenceBitmap) != optionalCount {
		return &ConstraintViolationError{
			TypeName:   "SEQUENCE",
			Value:      len(presenceBitmap),
			Constraint: "preamble bitmap size mismatch",
			Position:   se.encoder.stream.Position(),
		}
	}

	// Encode preamble bits
	for _, present := range presenceBitmap {
		if present {
			se.encoder.stream.WriteBit(1)
		} else {
			se.encoder.stream.WriteBit(0)
		}
	}

	return nil
}

// EncodeExtensionAdditions encodes the extension section per X.691 Clause 19.6.
// Extensions are encoded as "open types" with length determinants.
func (se *SequenceEncoder) EncodeExtensionAdditions(extensionPresence []bool, extensionValues [][]byte) error {
	if !se.constraints.Extensible {
		return nil
	}

	// Count present extensions
	presentCount := 0
	for _, present := range extensionPresence {
		if present {
			presentCount++
		}
	}

	if presentCount == 0 {
		return nil // No extensions to encode
	}

	// Validate extension count
	if len(extensionPresence) != len(se.constraints.ExtComponents) {
		return &ConstraintViolationError{
			TypeName:   "SEQUENCE",
			Value:      len(extensionPresence),
			Constraint: "extension bitmap size mismatch",
			Position:   se.encoder.stream.Position(),
		}
	}

	// X.691 §19.8: encode the bitmap width as a normally small length (lb=1).
	numExtensions := int64(len(se.constraints.ExtComponents))
	if err := se.encoder.EncodeNormallySmallLength(numExtensions); err != nil {
		return err
	}

	// Encode extension presence bitmap
	for _, present := range extensionPresence {
		if present {
			se.encoder.stream.WriteBit(1)
		} else {
			se.encoder.stream.WriteBit(0)
		}
	}

	// Encode extension values as open types
	extIdx := 0
	for _, present := range extensionPresence {
		if present {
			if extIdx >= len(extensionValues) {
				return &ConstraintViolationError{
					TypeName:   "SEQUENCE",
					Value:      extIdx,
					Constraint: "missing extension value",
					Position:   se.encoder.stream.Position(),
				}
			}

			value := extensionValues[extIdx]
			extIdx++

			// X.691 §19.9: extension addition value encoded as an open type (§11.2).
			if err := se.encoder.EncodeOpenType(value); err != nil {
				return err
			}
		}
	}

	return nil
}

// DecodeExtensionBit decodes the extension presence bit (if extensible).
func (sd *SequenceDecoder) DecodeExtensionBit() error {
	if !sd.constraints.Extensible {
		sd.hasExtensions = false
		return nil
	}

	extBit, err := sd.decoder.stream.ReadBit()
	if err != nil {
		return &DecodeError{
			TypeName: "SEQUENCE",
			Reason:   "failed to read extension bit",
			Position: sd.decoder.stream.Position(),
		}
	}

	sd.hasExtensions = (extBit == 1)
	return nil
}

// DecodePreamble decodes the preamble bitmap for optional/default root components.
func (sd *SequenceDecoder) DecodePreamble() error {
	// Count optional/default fields in root
	optionalCount := 0
	for _, comp := range sd.constraints.RootComponents {
		if comp.Optional || comp.HasDefault {
			optionalCount++
		}
	}

	// Decode preamble bits
	sd.optionalBitmap = make([]bool, optionalCount)
	for i := 0; i < optionalCount; i++ {
		bit, err := sd.decoder.stream.ReadBit()
		if err != nil {
			return &DecodeError{
				TypeName: "SEQUENCE",
				Reason:   "failed to read preamble bit",
				Position: sd.decoder.stream.Position(),
			}
		}
		sd.optionalBitmap[i] = (bit == 1)
	}

	return nil
}

// IsComponentPresent returns whether a root component is present based on the preamble.
func (sd *SequenceDecoder) IsComponentPresent(componentIndex int) bool {
	comp := sd.constraints.RootComponents[componentIndex]

	// Mandatory components are always present
	if !comp.Optional && !comp.HasDefault {
		return true
	}

	// Find index in optional bitmap
	optIdx := 0
	for i := 0; i < componentIndex; i++ {
		if sd.constraints.RootComponents[i].Optional || sd.constraints.RootComponents[i].HasDefault {
			optIdx++
		}
	}

	if optIdx >= len(sd.optionalBitmap) {
		return false
	}

	return sd.optionalBitmap[optIdx]
}

// DecodeExtensionAdditions decodes the extension section per X.691 Clause 19.6.
func (sd *SequenceDecoder) DecodeExtensionAdditions() ([][]byte, error) {
	if !sd.hasExtensions {
		return nil, nil
	}

	// X.691 §19.8: decode the bitmap width as a normally small length (lb=1).
	numExtensions, err := sd.decoder.DecodeNormallySmallLength()
	if err != nil {
		return nil, err
	}

	sd.extensionCount = numExtensions

	// Decode extension presence bitmap
	sd.extensionBitmap = make([]bool, numExtensions)
	for i := int64(0); i < numExtensions; i++ {
		bit, err := sd.decoder.stream.ReadBit()
		if err != nil {
			return nil, &DecodeError{
				TypeName: "SEQUENCE",
				Reason:   "failed to read extension bitmap",
				Position: sd.decoder.stream.Position(),
			}
		}
		sd.extensionBitmap[i] = (bit == 1)
	}

	// X.691 §19.9: decode extension addition values as open types (§11.2).
	extensionValues := make([][]byte, 0)
	for i := int64(0); i < numExtensions; i++ {
		if sd.extensionBitmap[i] {
			value, err := sd.decoder.DecodeOpenType()
			if err != nil {
				return nil, err
			}
			extensionValues = append(extensionValues, value)
		}
	}

	return extensionValues, nil
}

func (sd *SequenceDecoder) IsExtensionPresent(index int) bool { /////////////////////////////////////
	if index < 0 || index >= len(sd.extensionBitmap) {
		return false
	}
	return sd.extensionBitmap[index]
}

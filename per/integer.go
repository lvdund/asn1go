package per

import (
	"math"
	"math/big"
)

// EncodeInteger encodes an INTEGER value per X.691 Clauses 13-14.
// The encoding varies significantly based on constraint type:
// - Single value (lb == ub): no bits emitted
// - Constrained (R ≤ 255): fixed bits for range
// - Constrained (R = 256): 1 octet, APER aligned
// - Constrained (257 ≤ R ≤ 64K): 2 octets, APER aligned
// - Constrained (R > 64K): length determinant + minimum octets
// - Semi-constrained (lb only): length determinant + offset octets
// - Unconstrained: length determinant + 2's complement octets
func (e *Encoder) EncodeInteger(value int64, constraints IntegerConstraints) error {
	// X.691 Clause 13.2.1: Check for extensibility
	if constraints.Extensible {
		// Determine if value is in root or extension
		inRoot := false
		if constraints.LowerBound != nil && constraints.UpperBound != nil {
			inRoot = value >= *constraints.LowerBound && value <= *constraints.UpperBound
		}

		if inRoot {
			// Extension bit: 0 = in root
			e.stream.WriteBit(0)
			return e.encodeConstrainedInteger(value, constraints, true)
		} else {
			// Extension bit: 1 = in extension
			e.stream.WriteBit(1)
			return e.encodeUnconstrainedInteger(value)
		}
	}

	// No extension marker - encode normally
	return e.encodeConstrainedInteger(value, constraints, false)
}

// encodeConstrainedInteger handles constrained integer encoding per X.691 Clause 13.2.
func (e *Encoder) encodeConstrainedInteger(value int64, constraints IntegerConstraints, isRoot bool) error {
	lb := constraints.LowerBound
	ub := constraints.UpperBound

	// X.691 Clause 13.2.1: Single value constraint (lb == ub)
	if lb != nil && ub != nil && *lb == *ub {
		// Single value: no bits emitted
		if value != *lb {
			return &ConstraintViolationError{
				TypeName:   "INTEGER",
				Value:      value,
				Constraint: "single value",
				Position:   e.stream.Position(),
			}
		}
		return nil
	}

	// X.691 Clause 13.2.2: Constrained whole number (both bounds present)
	if lb != nil && ub != nil {
		return e.encodeConstrainedWholeNumber(value, *lb, *ub)
	}

	// X.691 Clause 13.2.3: Semi-constrained integer (lower bound only)
	if lb != nil && ub == nil {
		return e.encodeSemiConstrainedInteger(value, *lb)
	}

	// X.691 Clause 13.2.4: Unconstrained integer (no bounds)
	return e.encodeUnconstrainedInteger(value)
}

// encodeConstrainedWholeNumber encodes INTEGER with both upper and lower bounds per X.691 Clause 13.2.2.
func (e *Encoder) encodeConstrainedWholeNumber(value, lb, ub int64) error {
	// Validate value in range
	if value < lb || value > ub {
		return &ConstraintViolationError{
			TypeName:   "INTEGER",
			Value:      value,
			Constraint: "outside bounds",
			Position:   e.stream.Position(),
		}
	}

	r := ub - lb + 1 // Range size
	offset := uint64(value - lb)

	// X.691 Clause 13.2.2.2: Small range (R ≤ 255)
	if r <= 255 {
		bits := getFixedBitsForRange(r)
		e.stream.WriteBits(offset, bits)
		return nil
	}

	// X.691 Clause 13.2.2.3: Range exactly 256
	if r == 256 {
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		e.stream.WriteBits(offset, 8)
		return nil
	}

	// X.691 Clause 13.2.2.4: Range 257-65536
	if r <= 65536 {
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		e.stream.WriteBits(offset>>8, 8)   // High byte
		e.stream.WriteBits(offset&0xFF, 8) // Low byte
		return nil
	}

	// X.691 Clause 13.2.2.5: Large range (R > 64K)
	//
	// UPER: a fixed bit-field of ceil(log2(range)) bits. No length determinant.
	if e.variant == UPER {
		e.stream.WriteBits(offset, bitsNeeded(uint64(r)))
		return nil
	}

	// APER: a constrained length determinant (1..maxOctets, NOT octet-aligned),
	// then octet-alignment, then the minimum-octet value.
	octets := encodeToMinOctetsUnsigned(offset)
	maxOctets := calculateMaxOctets(lb, ub)
	if err := e.EncodeLengthDeterminant(int64(len(octets)), SizeRange(1, maxOctets)); err != nil {
		return err
	}
	e.stream.AlignToOctet()

	// Write octets
	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeSemiConstrainedInteger encodes INTEGER with lower bound only per X.691 Clause 13.2.3.
func (e *Encoder) encodeSemiConstrainedInteger(value, lb int64) error {
	if value < lb {
		return &ConstraintViolationError{
			TypeName:   "INTEGER",
			Value:      value,
			Constraint: "below lower bound",
			Position:   e.stream.Position(),
		}
	}

	offset := uint64(value - lb)
	octets := encodeToMinOctetsUnsigned(offset)

	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Length determinant (unconstrained)
	constraints := SizeConstraints{Min: nil, Max: nil}
	if err := e.EncodeLengthDeterminant(int64(len(octets)), constraints); err != nil {
		return err
	}

	// Write octets
	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeUnconstrainedInteger encodes INTEGER with no constraints per X.691 Clause 13.2.4.
func (e *Encoder) encodeUnconstrainedInteger(value int64) error {
	octets := encodeToMinOctetsSigned(value)

	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Length determinant (unconstrained)
	constraints := SizeConstraints{Min: nil, Max: nil}
	if err := e.EncodeLengthDeterminant(int64(len(octets)), constraints); err != nil {
		return err
	}

	// Write octets
	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeConstrainedIntegerBig handles constrained big.Int encoding per
// X.691 Clause 13.2, dispatching on which bounds are present.
func (e *Encoder) encodeConstrainedIntegerBig(value, lb, ub *big.Int) error {
	// X.691 Clause 13.2.1: Single value constraint (lb == ub)
	if lb != nil && ub != nil && lb.Cmp(ub) == 0 {
		if value.Cmp(lb) != 0 {
			return &ConstraintViolationError{
				TypeName:   "INTEGER",
				Value:      value,
				Constraint: "single value",
				Position:   e.stream.Position(),
			}
		}
		return nil
	}

	// X.691 Clause 13.2.2: Constrained whole number (both bounds present)
	if lb != nil && ub != nil {
		return e.encodeConstrainedWholeNumberBig(value, lb, ub)
	}

	// X.691 Clause 13.2.3: Semi-constrained integer (lower bound only)
	if lb != nil {
		return e.encodeSemiConstrainedIntegerBig(value, lb)
	}

	// X.691 Clause 13.2.4: Unconstrained integer (no bounds)
	return e.encodeUnconstrainedIntegerBig(value)
}

// encodeConstrainedWholeNumberBig encodes a constrained INTEGER with big.Int
// bounds per X.691 Clause 13.2.2. Range buckets mirror the int64 path so that
// in-range values produce byte-identical output to EncodeInteger.
func (e *Encoder) encodeConstrainedWholeNumberBig(value, lb, ub *big.Int) error {
	if value.Cmp(lb) < 0 || value.Cmp(ub) > 0 {
		return &ConstraintViolationError{
			TypeName:   "INTEGER",
			Value:      value,
			Constraint: "outside bounds",
			Position:   e.stream.Position(),
		}
	}

	r := new(big.Int).Sub(ub, lb)
	r.Add(r, big.NewInt(1)) // R = ub - lb + 1 (range size)

	offset := new(big.Int).Sub(value, lb) // >= 0

	// X.691 Clause 13.2.2.2: Small range (R <= 255)
	if r.IsUint64() && r.Uint64() <= 255 {
		bits := getFixedBitsForRange(int64(r.Uint64()))
		e.stream.WriteBits(offset.Uint64(), bits)
		return nil
	}

	// X.691 Clause 13.2.2.3: Range exactly 256
	if r.IsUint64() && r.Uint64() == 256 {
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		e.stream.WriteBits(offset.Uint64(), 8)
		return nil
	}

	// X.691 Clause 13.2.2.4: Range 257-65536
	if r.IsUint64() && r.Uint64() <= 65536 {
		if e.variant == APER {
			e.stream.AlignToOctet()
		}
		o := offset.Uint64()
		e.stream.WriteBits(o>>8, 8)   // High byte
		e.stream.WriteBits(o&0xFF, 8) // Low byte
		return nil
	}

	// X.691 Clause 13.2.2.5: Large range (R > 64K)
	//
	// UPER: a fixed bit-field of ceil(log2(range)) bits. No length determinant.
	if e.variant == UPER {
		bits := bitsNeededBig(r)
		if offset.IsUint64() && bits <= 64 {
			e.stream.WriteBits(offset.Uint64(), bits)
		} else {
			writeBitsBig(e.stream, offset, bits)
		}
		return nil
	}

	// APER: a constrained length determinant (1..maxOctets, NOT octet-aligned),
	// then octet-alignment, then the minimum-octet value.
	octets := bigEncodeToMinOctetsUnsigned(offset)
	maxOctets := bigMaxOctets(lb, ub)
	if err := e.EncodeLengthDeterminant(int64(len(octets)), SizeRange(1, maxOctets)); err != nil {
		return err
	}
	e.stream.AlignToOctet()

	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeSemiConstrainedIntegerBig encodes a big INTEGER with lower bound only
// per X.691 Clause 13.2.3.
func (e *Encoder) encodeSemiConstrainedIntegerBig(value, lb *big.Int) error {
	if value.Cmp(lb) < 0 {
		return &ConstraintViolationError{
			TypeName:   "INTEGER",
			Value:      value,
			Constraint: "below lower bound",
			Position:   e.stream.Position(),
		}
	}

	offset := new(big.Int).Sub(value, lb)
	octets := bigEncodeToMinOctetsUnsigned(offset)

	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	if err := e.EncodeLengthDeterminant(int64(len(octets)), SizeConstraints{Min: nil, Max: nil}); err != nil {
		return err
	}

	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// encodeUnconstrainedIntegerBig encodes a big INTEGER with no constraints per
// X.691 Clause 13.2.4 (2's complement).
func (e *Encoder) encodeUnconstrainedIntegerBig(value *big.Int) error {
	octets := bigEncodeToMinOctetsSigned(value)

	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	if err := e.EncodeLengthDeterminant(int64(len(octets)), SizeConstraints{Min: nil, Max: nil}); err != nil {
		return err
	}

	for _, octet := range octets {
		e.stream.WriteBits(uint64(octet), 8)
	}

	return nil
}

// EncodeBigInteger encodes an INTEGER using math/big.Int, supporting bounds
// and values outside the int64 range (e.g. INTEGER (0..2^64-1)). The encoding
// mirrors EncodeInteger per X.691 Clauses 13-14; effective bounds are taken
// from BigIntLower/BigIntUpper when set, otherwise from the int64 fields.
func (e *Encoder) EncodeBigInteger(value *big.Int, constraints IntegerConstraints) error {
	if value == nil {
		value = big.NewInt(0)
	}
	lb := constraints.bigLowerBound()
	ub := constraints.bigUpperBound()

	// X.691 Clause 13.2.1: extensibility
	if constraints.Extensible {
		inRoot := false
		if lb != nil && ub != nil {
			inRoot = value.Cmp(lb) >= 0 && value.Cmp(ub) <= 0
		}
		if inRoot {
			e.stream.WriteBit(0)
			return e.encodeConstrainedIntegerBig(value, lb, ub)
		}
		e.stream.WriteBit(1)
		return e.encodeUnconstrainedIntegerBig(value)
	}

	return e.encodeConstrainedIntegerBig(value, lb, ub)
}

// DecodeInteger decodes an INTEGER value per X.691 Clauses 13-14.
func (d *Decoder) DecodeInteger(constraints IntegerConstraints) (int64, error) {
	// X.691 Clause 13.2.1: Check for extensibility
	if constraints.Extensible {
		// Read extension bit
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}

		if extBit == 0 {
			// In root: decode as constrained
			return d.decodeConstrainedInteger(constraints, true)
		} else {
			// In extension: decode as unconstrained
			return d.decodeUnconstrainedInteger()
		}
	}

	// No extension marker
	return d.decodeConstrainedInteger(constraints, false)
}

// decodeConstrainedInteger handles constrained integer decoding.
func (d *Decoder) decodeConstrainedInteger(constraints IntegerConstraints, isRoot bool) (int64, error) {
	lb := constraints.LowerBound
	ub := constraints.UpperBound

	// Single value constraint
	if lb != nil && ub != nil && *lb == *ub {
		return *lb, nil
	}

	// Constrained whole number
	if lb != nil && ub != nil {
		return d.decodeConstrainedWholeNumber(*lb, *ub)
	}

	// Semi-constrained
	if lb != nil && ub == nil {
		return d.decodeSemiConstrainedInteger(*lb)
	}

	// Unconstrained
	return d.decodeUnconstrainedInteger()
}

// decodeConstrainedWholeNumber decodes INTEGER with both bounds.
func (d *Decoder) decodeConstrainedWholeNumber(lb, ub int64) (int64, error) {
	r := ub - lb + 1

	// Small range (R ≤ 255)
	if r <= 255 {
		bits := getFixedBitsForRange(r)
		offset, err := d.stream.ReadBits(bits)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read constrained integer",
				Position: d.stream.Position(),
			}
		}
		return lb + int64(offset), nil
	}

	// Range exactly 256
	if r == 256 {
		if d.variant == APER {
			// Read alignment padding
			for d.stream.Position()%8 != 0 {
				_, err := d.stream.ReadBit()
				if err != nil {
					return 0, err
				}
			}
		}
		offset, err := d.stream.ReadBits(8)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer octet",
				Position: d.stream.Position(),
			}
		}
		return lb + int64(offset), nil
	}

	// Range 257-65536
	if r <= 65536 {
		if d.variant == APER {
			// Read alignment padding
			for d.stream.Position()%8 != 0 {
				_, err := d.stream.ReadBit()
				if err != nil {
					return 0, err
				}
			}
		}
		high, err := d.stream.ReadBits(8)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer high byte",
				Position: d.stream.Position(),
			}
		}
		low, err := d.stream.ReadBits(8)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer low byte",
				Position: d.stream.Position(),
			}
		}
		offset := (high << 8) | low
		return lb + int64(offset), nil
	}

	// Large range (R > 64K)
	//
	// UPER: a fixed bit-field of ceil(log2(range)) bits.
	if d.variant == UPER {
		offset, err := d.stream.ReadBits(bitsNeeded(uint64(r)))
		if err != nil {
			return 0, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read large-range integer",
				Position: d.stream.Position(),
			}
		}
		return lb + int64(offset), nil
	}

	// APER: constrained length determinant, then octet-alignment, then octets.
	maxOctets := calculateMaxOctets(lb, ub)
	length, err := d.DecodeLengthDeterminant(SizeRange(1, maxOctets))
	if err != nil {
		return 0, err
	}
	for d.stream.Position()%8 != 0 {
		if _, err := d.stream.ReadBit(); err != nil {
			return 0, err
		}
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return 0, err
	}

	offset := decodeFromOctetsUnsigned(octets)
	return lb + int64(offset), nil
}

// decodeSemiConstrainedInteger decodes INTEGER with lower bound only.
func (d *Decoder) decodeSemiConstrainedInteger(lb int64) (int64, error) {
	if d.variant == APER {
		// Read alignment padding
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return 0, err
			}
		}
	}

	constraints := SizeConstraints{Min: nil, Max: nil}
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return 0, err
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return 0, err
	}

	offset := decodeFromOctetsUnsigned(octets)
	return lb + int64(offset), nil
}

// decodeUnconstrainedInteger decodes INTEGER with no constraints.
func (d *Decoder) decodeUnconstrainedInteger() (int64, error) {
	if d.variant == APER {
		// Read alignment padding
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return 0, err
			}
		}
	}

	constraints := SizeConstraints{Min: nil, Max: nil}
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return 0, err
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return 0, err
	}

	return decodeFromOctetsSigned(octets), nil
}

// DecodeBigInteger decodes an INTEGER using math/big.Int, supporting bounds
// and values outside the int64 range. Mirrors DecodeInteger per X.691 Clauses
// 13-14; effective bounds come from BigIntLower/BigIntUpper when set.
func (d *Decoder) DecodeBigInteger(constraints IntegerConstraints) (*big.Int, error) {
	lb := constraints.bigLowerBound()
	ub := constraints.bigUpperBound()

	// X.691 Clause 13.2.1: extensibility
	if constraints.Extensible {
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}
		if extBit == 0 {
			return d.decodeConstrainedIntegerBig(lb, ub)
		}
		return d.decodeUnconstrainedIntegerBig()
	}

	return d.decodeConstrainedIntegerBig(lb, ub)
}

// decodeConstrainedIntegerBig dispatches constrained big.Int decoding.
func (d *Decoder) decodeConstrainedIntegerBig(lb, ub *big.Int) (*big.Int, error) {
	// Single value constraint
	if lb != nil && ub != nil && lb.Cmp(ub) == 0 {
		return new(big.Int).Set(lb), nil
	}

	// Constrained whole number
	if lb != nil && ub != nil {
		return d.decodeConstrainedWholeNumberBig(lb, ub)
	}

	// Semi-constrained
	if lb != nil {
		return d.decodeSemiConstrainedIntegerBig(lb)
	}

	// Unconstrained
	return d.decodeUnconstrainedIntegerBig()
}

// decodeConstrainedWholeNumberBig decodes a constrained big INTEGER with both
// bounds present per X.691 Clause 13.2.2.
func (d *Decoder) decodeConstrainedWholeNumberBig(lb, ub *big.Int) (*big.Int, error) {
	r := new(big.Int).Sub(ub, lb)
	r.Add(r, big.NewInt(1)) // R = ub - lb + 1

	// Small range (R <= 255)
	if r.IsUint64() && r.Uint64() <= 255 {
		bits := getFixedBitsForRange(int64(r.Uint64()))
		offset, err := d.stream.ReadBits(bits)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read constrained integer",
				Position: d.stream.Position(),
			}
		}
		return new(big.Int).Add(lb, new(big.Int).SetUint64(offset)), nil
	}

	// Range exactly 256
	if r.IsUint64() && r.Uint64() == 256 {
		if d.variant == APER {
			for d.stream.Position()%8 != 0 {
				if _, err := d.stream.ReadBit(); err != nil {
					return nil, err
				}
			}
		}
		offset, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer octet",
				Position: d.stream.Position(),
			}
		}
		return new(big.Int).Add(lb, new(big.Int).SetUint64(offset)), nil
	}

	// Range 257-65536
	if r.IsUint64() && r.Uint64() <= 65536 {
		if d.variant == APER {
			for d.stream.Position()%8 != 0 {
				if _, err := d.stream.ReadBit(); err != nil {
					return nil, err
				}
			}
		}
		high, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer high byte",
				Position: d.stream.Position(),
			}
		}
		low, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read integer low byte",
				Position: d.stream.Position(),
			}
		}
		offset := (high << 8) | low
		return new(big.Int).Add(lb, new(big.Int).SetUint64(offset)), nil
	}

	// Large range (R > 64K)
	//
	// UPER: a fixed bit-field of ceil(log2(range)) bits.
	if d.variant == UPER {
		bits := bitsNeededBig(r)
		offset, err := readBitsBig(d.stream, bits)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read large-range integer",
				Position: d.stream.Position(),
			}
		}
		return new(big.Int).Add(lb, offset), nil
	}

	// APER: constrained length determinant, then octet-alignment, then octets.
	maxOctets := bigMaxOctets(lb, ub)
	length, err := d.DecodeLengthDeterminant(SizeRange(1, maxOctets))
	if err != nil {
		return nil, err
	}
	for d.stream.Position()%8 != 0 {
		if _, err := d.stream.ReadBit(); err != nil {
			return nil, err
		}
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return nil, err
	}

	offset := bigDecodeFromOctetsUnsigned(octets)
	return new(big.Int).Add(lb, offset), nil
}

// decodeSemiConstrainedIntegerBig decodes a big INTEGER with lower bound only.
func (d *Decoder) decodeSemiConstrainedIntegerBig(lb *big.Int) (*big.Int, error) {
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			if _, err := d.stream.ReadBit(); err != nil {
				return nil, err
			}
		}
	}

	length, err := d.DecodeLengthDeterminant(SizeConstraints{Min: nil, Max: nil})
	if err != nil {
		return nil, err
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return nil, err
	}

	offset := bigDecodeFromOctetsUnsigned(octets)
	return new(big.Int).Add(lb, offset), nil
}

// decodeUnconstrainedIntegerBig decodes a big INTEGER with no constraints
// (2's complement).
func (d *Decoder) decodeUnconstrainedIntegerBig() (*big.Int, error) {
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			if _, err := d.stream.ReadBit(); err != nil {
				return nil, err
			}
		}
	}

	length, err := d.DecodeLengthDeterminant(SizeConstraints{Min: nil, Max: nil})
	if err != nil {
		return nil, err
	}

	octets, err := d.readOctets(int(length))
	if err != nil {
		return nil, err
	}

	return bigDecodeFromOctetsSigned(octets), nil
}

// readOctets reads n octets from the stream.
func (d *Decoder) readOctets(n int) ([]byte, error) {
	octets := make([]byte, n)
	for i := range n {
		octet, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "INTEGER",
				Reason:   "failed to read octet",
				Position: d.stream.Position(),
			}
		}
		octets[i] = byte(octet)
	}
	return octets, nil
}

// Helper functions

// getFixedBitsForRange returns the minimum number of bits needed for a range per X.691.
func getFixedBitsForRange(r int64) int {
	if r <= 1 {
		return 0
	}
	if r == 2 {
		return 1
	}
	if r <= 4 {
		return 2
	}
	if r <= 8 {
		return 3
	}
	if r <= 16 {
		return 4
	}
	if r <= 32 {
		return 5
	}
	if r <= 64 {
		return 6
	}
	if r <= 128 {
		return 7
	}
	// r <= 255
	return 8
}

// calculateMaxOctets calculates maximum octets needed for a range.
func calculateMaxOctets(lb, ub int64) int64 {
	maxValue := ub - lb
	if maxValue <= 0 {
		return 1
	}
	// Calculate octets needed to represent maxValue
	octets := int64(0)
	for maxValue > 0 {
		octets++
		maxValue >>= 8
	}
	return octets
}

// encodeToMinOctetsUnsigned encodes an unsigned value in minimum octets.
func encodeToMinOctetsUnsigned(value uint64) []byte {
	if value == 0 {
		return []byte{0}
	}

	// Count octets needed
	var octets []byte
	for value > 0 {
		octets = append([]byte{byte(value & 0xFF)}, octets...)
		value >>= 8
	}
	return octets
}

// encodeToMinOctetsSigned encodes a signed value in minimum octets (2's complement).
func encodeToMinOctetsSigned(value int64) []byte {
	if value == 0 {
		return []byte{0}
	}

	// Determine if negative
	isNegative := value < 0

	// Convert to unsigned for processing
	var uvalue uint64
	if isNegative {
		uvalue = uint64(value)
	} else {
		uvalue = uint64(value)
	}

	// Encode octets
	var octets []byte
	for {
		octet := byte(uvalue & 0xFF)
		octets = append([]byte{octet}, octets...)
		uvalue >>= 8

		// Check if we need more octets
		if uvalue == 0 && !isNegative && (octet&0x80) == 0 {
			break
		}
		if uvalue == math.MaxUint64 && isNegative && (octet&0x80) != 0 {
			break
		}
		if uvalue == 0 || uvalue == math.MaxUint64 {
			break
		}
	}

	return octets
}

// decodeFromOctetsUnsigned decodes unsigned value from octets.
func decodeFromOctetsUnsigned(octets []byte) uint64 {
	var value uint64
	for _, octet := range octets {
		value = (value << 8) | uint64(octet)
	}
	return value
}

// decodeFromOctetsSigned decodes signed value from octets (2's complement).
func decodeFromOctetsSigned(octets []byte) int64 {
	if len(octets) == 0 {
		return 0
	}

	// Check if negative (MSB set)
	isNegative := (octets[0] & 0x80) != 0

	var value uint64
	for _, octet := range octets {
		value = (value << 8) | uint64(octet)
	}

	if isNegative {
		// Sign extend: convert from two's complement
		// For int64, shift left then arithmetic shift right
		shift := uint(64 - len(octets)*8)
		return int64(value<<shift) >> shift
	}

	return int64(value)
}

// ---------------------------------------------------------------------------
// big.Int support for INTEGER values/bounds outside the int64 range.
// ---------------------------------------------------------------------------

// bigLowerBound resolves the effective lower bound as a big.Int, preferring
// BigIntLower and falling back to the int64 LowerBound.
func (c IntegerConstraints) bigLowerBound() *big.Int {
	if c.BigIntLower != nil {
		return c.BigIntLower
	}
	if c.LowerBound != nil {
		return big.NewInt(*c.LowerBound)
	}
	return nil
}

// bigUpperBound resolves the effective upper bound as a big.Int, preferring
// BigIntUpper and falling back to the int64 UpperBound.
func (c IntegerConstraints) bigUpperBound() *big.Int {
	if c.BigIntUpper != nil {
		return c.BigIntUpper
	}
	if c.UpperBound != nil {
		return big.NewInt(*c.UpperBound)
	}
	return nil
}

// bitsNeededBig returns ceil(log2(r)) for r > 0 (0 for r <= 0): the number of
// bits needed to enumerate offsets in [0, r-1]. Mirrors bitsNeeded but for
// ranges that may exceed uint64.
func bitsNeededBig(r *big.Int) int {
	if r.Sign() <= 0 {
		return 0
	}
	bl := r.BitLen() // 2^k has BitLen k+1
	if isPowerOfTwoBig(r) {
		return bl - 1
	}
	return bl
}

// isPowerOfTwoBig reports whether r is a positive power of two.
func isPowerOfTwoBig(r *big.Int) bool {
	if r.Sign() <= 0 {
		return false
	}
	one := big.NewInt(1)
	tmp := new(big.Int).Sub(r, one)
	tmp.And(tmp, r)
	return tmp.Sign() == 0
}

// bigMaxOctets returns the number of octets needed to represent the largest
// offset (ub - lb) for a constrained large-range INTEGER. Mirrors
// calculateMaxOctets.
func bigMaxOctets(lb, ub *big.Int) int64 {
	maxOffset := new(big.Int).Sub(ub, lb)
	if maxOffset.Sign() <= 0 {
		return 1
	}
	return int64((maxOffset.BitLen() + 7) / 8)
}

// bigEncodeToMinOctetsUnsigned encodes a non-negative big.Int in minimum
// big-endian octets (at least one). Mirrors encodeToMinOctetsUnsigned.
func bigEncodeToMinOctetsUnsigned(value *big.Int) []byte {
	if value.Sign() == 0 {
		return []byte{0}
	}
	return value.Bytes() // big-endian, minimal, no leading zeros
}

// bigDecodeFromOctetsUnsigned decodes an unsigned big.Int from big-endian
// octets. Mirrors decodeFromOctetsUnsigned.
func bigDecodeFromOctetsUnsigned(octets []byte) *big.Int {
	return new(big.Int).SetBytes(octets)
}

// bigEncodeToMinOctetsSigned encodes a big.Int in minimum-octet two's
// complement. Mirrors encodeToMinOctetsSigned but for arbitrary precision.
// The smallest k is chosen such that value lies in [-2^(8k-1), 2^(8k-1)-1].
func bigEncodeToMinOctetsSigned(value *big.Int) []byte {
	if value.Sign() == 0 {
		return []byte{0}
	}
	for k := 1; ; k++ {
		halfMod := new(big.Int).Lsh(big.NewInt(1), uint(8*k-1)) // 2^(8k-1)
		mod := new(big.Int).Lsh(halfMod, 1)                     // 2^(8k)
		upper := new(big.Int).Sub(halfMod, big.NewInt(1))       // 2^(8k-1) - 1
		lower := new(big.Int).Neg(halfMod)                      // -2^(8k-1)
		if value.Cmp(lower) >= 0 && value.Cmp(upper) <= 0 {
			res := new(big.Int).Mod(value, mod) // truncated; may be negative
			if res.Sign() < 0 {
				res.Add(res, mod)
			}
			out := make([]byte, k)
			res.FillBytes(out)
			return out
		}
	}
}

// bigDecodeFromOctetsSigned decodes a two's-complement signed big.Int from
// big-endian octets. Mirrors decodeFromOctetsSigned.
func bigDecodeFromOctetsSigned(octets []byte) *big.Int {
	v := new(big.Int).SetBytes(octets)
	if len(octets) > 0 && octets[0]&0x80 != 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), uint(8*len(octets)))
		v.Sub(v, mod)
	}
	return v
}

// writeBitsBig writes 'count' bits of value to the stream, MSB first, masking
// to count bits. Used when count exceeds 64 (WriteBits is capped at 64).
func writeBitsBig(s *BitStream, value *big.Int, count int) {
	if count == 0 {
		return
	}
	v := new(big.Int).Set(value)
	if v.BitLen() > count { // mask off any bits above count
		mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(count)), big.NewInt(1))
		v.And(v, mask)
	}
	for i := count - 1; i >= 0; i-- {
		s.WriteBit(int(v.Bit(i)))
	}
}

// readBitsBig reads 'count' bits from the stream into a big.Int, MSB first.
// Used when count exceeds 64 (ReadBits is capped at 64).
func readBitsBig(s *BitStream, count int) (*big.Int, error) {
	if count <= 64 {
		v, err := s.ReadBits(count)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetUint64(v), nil
	}
	result := new(big.Int)
	for range count {
		bit, err := s.ReadBit()
		if err != nil {
			return nil, err
		}
		result.Lsh(result, 1)
		if bit == 1 {
			result.Or(result, big.NewInt(1))
		}
	}
	return result, nil
}

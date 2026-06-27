package per

import "math"

// EncodeLengthDeterminant encodes a length value per X.691 Clause 11.9.
//
// It is a dispatcher that selects the correct §11.9.3.1 category from the
// PER-visible size constraints:
//
//   - Constrained length (ub < 64K): §11.9.3.3 → §11.5 constrained whole
//     number (a fixed-width bit-field of ceil(log2(range)) bits).
//   - Unconstrained / semi-constrained / ub >= 64K: §11.9.3.5 → §11.9.3.6/7/8
//     via EncodeUnconstrainedLength.
//
// Callers that know their category should call the specific function:
//
//   - Open-type payloads (§11.2), OID/RELATIVE-OID (§24), REAL (§15),
//     unconstrained INTEGER (§13.2.4), semi-constrained INTEGER (§13.2.3),
//     unbounded OCTET STRING / BIT STRING / character strings, and extension
//     addition values (§19.9, §23.8): EncodeUnconstrainedLength.
//   - SEQUENCE/SET extension bitmap (§19.8): EncodeNormallySmallLength.
//
// NOTE: Previously this function routed the Max==nil path through the
// §11.9.3.4 "normally small length" procedure, which X.691 reserves
// exclusively for the extension bitmap. That produced malformed PER for every
// unconstrained/semi-constrained length < 64 octets (a phantom discriminator
// bit, wrong alignment, and an off-by-one value). The dispatcher now sends
// those lengths to EncodeUnconstrainedLength as the spec requires.
func (e *Encoder) EncodeLengthDeterminant(n int64, constraints SizeConstraints) error {
	// §11.9.3.3: Constrained length determinant (ub < 64K) → §11.5
	// constrained whole number rules, including APER octet alignment for
	// ranges 256..65536.
	if constraints.Max != nil && *constraints.Max < 65536 {
		min := int64(0)
		if constraints.Min != nil {
			min = *constraints.Min
		}
		max := *constraints.Max
		return e.encodeConstrainedWholeNumber(n, min, max)
	}

	// §11.9.3.5 → §11.9.3.6/7/8.
	return e.EncodeUnconstrainedLength(n)
}

// EncodeUnconstrainedLength encodes an unconstrained / semi-constrained length
// determinant per X.691 §11.9.3.5 → §11.9.3.6/7/8.
//
// Encoding (value "n" in octets):
//
//   - n <= 127 (§11.9.3.6): single octet 0xxxxxxx (n in bits 7..1, bit 8 = 0).
//   - 128 <= n < 16K (§11.9.3.7): two octets 10xxxxxx xxxxxxxx.
//   - n >= 16K (§11.9.3.8): fragmentation — NOT YET IMPLEMENTED.
//
// In the ALIGNED variant the length octet(s) are octet-aligned before being
// written (§11.9.3.6/7 "octet-aligned in the ALIGNED variant"). In the
// UNALIGNED variant the same octets are emitted with no alignment
// (§11.9.4.2 → §11.9.3.6/7/8).
func (e *Encoder) EncodeUnconstrainedLength(n int64) error {
	if n < 0 {
		return &ConstraintViolationError{
			TypeName:   "LENGTH",
			Value:      n,
			Constraint: "length must be non-negative",
			Position:   e.stream.Position(),
		}
	}

	// APER: the length octet is octet-aligned (§11.9.3.6/7).
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// §11.9.3.6: n <= 127 → single octet 0xxxxxxx.
	if n <= 127 {
		e.stream.WriteBits(uint64(n), 8)
		return nil
	}

	// §11.9.3.7: 128 <= n < 16K → two octets 10xxxxxx xxxxxxxx.
	if n < 16384 {
		e.stream.WriteBits(0x80|uint64(n>>8), 8)
		e.stream.WriteBits(uint64(n&0xFF), 8)
		return nil
	}

	// §11.9.3.8: n >= 16K → fragmentation. Not yet implemented; the previous
	// implementation also errored here, so this is not a regression.
	return &UnsupportedTypeError{
		TypeName: "LENGTH",
		Message:  "fragmentation not implemented (length >= 16384)",
	}
}

// EncodeNormallySmallLength encodes a normally small length (lb=1) per
// X.691 §11.9.3.4.
//
// RESERVED for the SEQUENCE/SET extension bitmap (§19.8). The NOTE in
// §11.9.3.4 states: "Normally small lengths are only used to indicate the
// length of the bitmap that prefixes the extension addition values of a set
// or sequence type." Do not use this for any other length.
//
// Encoding:
//   - n <= 64: bit 0 + 6-bit (n-1)   [lb=1, so the offset is n-1]
//   - n > 64:  bit 1 + EncodeUnconstrainedLength(n)
func (e *Encoder) EncodeNormallySmallLength(n int64) error {
	if n < 1 {
		return &ConstraintViolationError{
			TypeName:   "LENGTH",
			Value:      n,
			Constraint: "normally small length lower bound is 1",
			Position:   e.stream.Position(),
		}
	}

	if n <= 64 {
		e.stream.WriteBit(0)
		e.stream.WriteBits(uint64(n-1), 6)
		return nil
	}

	e.stream.WriteBit(1)
	return e.EncodeUnconstrainedLength(n)
}

// DecodeLengthDeterminant decodes a length value per X.691 Clause 11.9.
// Inverse of EncodeLengthDeterminant.
func (d *Decoder) DecodeLengthDeterminant(constraints SizeConstraints) (int64, error) {
	// §11.9.3.3: Constrained length determinant (ub < 64K) → §11.5
	// constrained whole number rules, including APER octet alignment for
	// ranges 256..65536.
	if constraints.Max != nil && *constraints.Max < 65536 {
		min := int64(0)
		if constraints.Min != nil {
			min = *constraints.Min
		}
		max := *constraints.Max
		return d.decodeConstrainedWholeNumber(min, max)
	}

	// §11.9.3.5 → §11.9.3.6/7/8.
	return d.DecodeUnconstrainedLength()
}

// DecodeUnconstrainedLength decodes an unconstrained / semi-constrained length
// determinant per X.691 §11.9.3.5 → §11.9.3.6/7/8.
// Inverse of EncodeUnconstrainedLength.
func (d *Decoder) DecodeUnconstrainedLength() (int64, error) {
	// APER: skip to octet boundary before the length octet.
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			if _, err := d.stream.ReadBit(); err != nil {
				return 0, &DecodeError{
					TypeName: "LENGTH",
					Reason:   "failed to read alignment padding",
					Position: d.stream.Position(),
				}
			}
		}
	}

	firstOctet, err := d.stream.ReadBits(8)
	if err != nil {
		return 0, &DecodeError{
			TypeName: "LENGTH",
			Reason:   "failed to read length determinant octet",
			Position: d.stream.Position(),
		}
	}

	// §11.9.3.6: bit 8 = 0 → single octet form.
	if (firstOctet & 0x80) == 0 {
		return int64(firstOctet), nil
	}

	// §11.9.3.7: bits 8,7 = 10 → two octet form.
	if (firstOctet & 0x40) == 0 {
		secondOctet, err := d.stream.ReadBits(8)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "LENGTH",
				Reason:   "failed to read second length octet",
				Position: d.stream.Position(),
			}
		}
		return int64((firstOctet&0x3F)<<8 | secondOctet), nil
	}

	// §11.9.3.8: bits 8,7 = 11 → fragmentation (not yet implemented).
	return 0, &DecodeError{
		TypeName: "LENGTH",
		Reason:   "fragmented length not yet implemented",
		Position: d.stream.Position(),
	}
}

// DecodeNormallySmallLength decodes a normally small length (lb=1) per
// X.691 §11.9.3.4. Inverse of EncodeNormallySmallLength. RESERVED for the
// SEQUENCE/SET extension bitmap (§19.8).
func (d *Decoder) DecodeNormallySmallLength() (int64, error) {
	flag, err := d.stream.ReadBit()
	if err != nil {
		return 0, &DecodeError{
			TypeName: "LENGTH",
			Reason:   "failed to read normally small length flag",
			Position: d.stream.Position(),
		}
	}

	if flag == 0 {
		value, err := d.stream.ReadBits(6)
		if err != nil {
			return 0, &DecodeError{
				TypeName: "LENGTH",
				Reason:   "failed to read normally small length value",
				Position: d.stream.Position(),
			}
		}
		return int64(value) + 1, nil
	}

	return d.DecodeUnconstrainedLength()
}

// bitsNeeded calculates the minimum number of bits needed to represent a range.
// Per X.691: ceil(log2(n))
func bitsNeeded(n uint64) int {
	if n <= 1 {
		return 0
	}
	return int(math.Ceil(math.Log2(float64(n))))
}

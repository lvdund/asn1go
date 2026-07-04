package per

import (
	"fmt"
	"math"
)

// EncodeReal encodes an ASN.1 REAL value (float64) per X.691 Clause 15.
//
// PER encodes REAL by:
// 1. Converting the float64 to binary format per ITU-T X.690 Clause 8.5
// 2. Encoding length as semi-constrained (lower bound = 0)
// 3. Writing content octets (with APER alignment before octets)
//
// Special values:
// - Zero (±0.0) → length 0
// - ±Infinity → special encoding (0x40 or 0x41)
// - NaN → not supported, returns error
func (e *Encoder) EncodeReal(value float64) error {
	// 1. Convert REAL to content octets
	contentOctets, err := realToBytes(value)
	if err != nil {
		return fmt.Errorf("REAL: %w", err)
	}

	// 2. Encode length (semi-constrained, lower bound = 0)
	n := int64(len(contentOctets))
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return fmt.Errorf("REAL: failed to encode length: %w", err)
	}

	// 3. Align for APER before writing octets (if length > 0)
	if n > 0 && e.variant == APER {
		e.stream.AlignToOctet()
	}

	// 4. Write content octets
	for _, b := range contentOctets {
		e.stream.WriteBits(uint64(b), 8)
	}

	return nil
}

// DecodeReal decodes an ASN.1 REAL value per X.691 Clause 15.
func (d *Decoder) DecodeReal() (float64, error) {
	// 1. Decode length (semi-constrained, lower bound = 0)
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	n, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return 0, fmt.Errorf("REAL: failed to decode length: %w", err)
	}

	// Zero length → value is zero
	if n == 0 {
		return 0.0, nil
	}

	// 2. Align for APER before reading octets
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return 0, fmt.Errorf("REAL: failed to align: %w", err)
			}
		}
	}

	// 3. Read content octets
	contentOctets := make([]byte, n)
	for i := int64(0); i < n; i++ {
		bits, err := d.stream.ReadBits(8)
		if err != nil {
			return 0, fmt.Errorf("REAL: failed to read octet at %d: %w", i, err)
		}
		contentOctets[i] = byte(bits)
	}

	// 4. Convert content octets back to float64
	value, err := bytesToReal(contentOctets)
	if err != nil {
		return 0, fmt.Errorf("REAL: failed to parse content: %w", err)
	}

	return value, nil
}

// realToBytes converts a float64 to REAL content octets per ITU-T X.690 Clause 8.5.
//
// Encoding format (binary encoding):
// - First octet: information octet (0x80 = binary, base 2)
// - Following octets: IEEE 754 double-precision encoding (8 bytes)
//
// Special cases:
// - Zero → empty (length 0)
// - +Infinity → 0x40
// - -Infinity → 0x41
// - NaN → error (not encodable)
func realToBytes(value float64) ([]byte, error) {
	// Special case: zero
	if value == 0.0 {
		// ITU-T X.690: zero is encoded as empty (length 0)
		return []byte{}, nil
	}

	// Special case: +Infinity
	if math.IsInf(value, 1) {
		return []byte{0x40}, nil
	}

	// Special case: -Infinity
	if math.IsInf(value, -1) {
		return []byte{0x41}, nil
	}

	// NaN is not encodable per X.690
	if math.IsNaN(value) {
		return nil, fmt.Errorf("NaN is not encodable")
	}

	// Binary encoding: ISO 6093 binary format
	// Use IEEE 754 double-precision (8 bytes)
	bits := math.Float64bits(value)

	// Binary encoding information octet: 0x80
	// Bit 7 = 1 (binary encoding)
	// Bits 6,5 = 00 (base = 2)
	// Bit 4 = 0 (scaling factor = 0)
	// Bits 3,2 = 00 (exponent format: follows in 1 octet)
	// Bits 1,0 = sign and format
	var result []byte
	result = append(result, 0x80) // Binary encoding, base 2

	// Append IEEE 754 bits as 8 octets (big-endian)
	for i := 7; i >= 0; i-- {
		octet := byte((bits >> uint(i*8)) & 0xFF)
		result = append(result, octet)
	}

	return result, nil
}

// bytesToReal converts REAL content octets to float64 per ITU-T X.690 Clause 8.5.
func bytesToReal(octets []byte) (float64, error) {
	if len(octets) == 0 {
		return 0.0, nil
	}

	// First octet is the information octet
	info := octets[0]

	// Special values
	if info == 0x40 {
		return math.Inf(1), nil // +Infinity
	}
	if info == 0x41 {
		return math.Inf(-1), nil // -Infinity
	}

	// Binary encoding check (bit 7 = 1)
	if info&0x80 != 0 {
		// Binary encoding: expect 9 octets total (1 info + 8 IEEE 754)
		if len(octets) != 9 {
			return 0, fmt.Errorf("binary REAL encoding: expected 9 octets, got %d", len(octets))
		}

		// Extract IEEE 754 bits (big-endian)
		var bits uint64
		for i := 1; i < 9; i++ {
			bits = (bits << 8) | uint64(octets[i])
		}

		return math.Float64frombits(bits), nil
	}

	// Decimal encoding (ISO 6093) not implemented
	return 0, fmt.Errorf("decimal REAL encoding not supported")
}

package per

import (
	"fmt"
	"strings"
)

// ObjectIdentifier represents an ASN.1 OBJECT IDENTIFIER value.
// It is stored as a sequence of integer components (arcs).
// X.691 Clause 24: OID is encoded as length + content octets.
type ObjectIdentifier struct {
	Arcs []uint64 // Component arcs (e.g., [1, 2, 840, 113549] for "1.2.840.113549")
}

// NewObjectIdentifier creates a new ObjectIdentifier from arc values.
func NewObjectIdentifier(arcs ...uint64) ObjectIdentifier {
	return ObjectIdentifier{Arcs: arcs}
}

// String returns the dot-notation string representation (e.g., "1.2.840.113549").
func (oid ObjectIdentifier) String() string {
	if len(oid.Arcs) == 0 {
		return ""
	}

	parts := make([]string, len(oid.Arcs))
	for i, arc := range oid.Arcs {
		parts[i] = fmt.Sprintf("%d", arc)
	}
	return strings.Join(parts, ".")
}

// Equal returns true if this OID equals another OID.
func (oid ObjectIdentifier) Equal(other ObjectIdentifier) bool {
	if len(oid.Arcs) != len(other.Arcs) {
		return false
	}
	for i := range oid.Arcs {
		if oid.Arcs[i] != other.Arcs[i] {
			return false
		}
	}
	return true
}

// EncodeObjectIdentifier encodes an OBJECT IDENTIFIER per X.691 Clause 24.
//
// PER encodes OID by:
// 1. Converting arcs to BER content octets:
//   - First byte = first_arc * 40 + second_arc (per X.690)
//   - Subsequent arcs encoded in base-128 (VLQ)
//
// 2. Encoding length as semi-constrained (lower bound = 0)
// 3. Writing content octets (with APER alignment before octets)
func (e *Encoder) EncodeObjectIdentifier(oid ObjectIdentifier) error {
	if len(oid.Arcs) < 2 {
		return fmt.Errorf("OID: must have at least 2 arcs, got %d", len(oid.Arcs))
	}

	// Validate first arc (must be 0, 1, or 2)
	if oid.Arcs[0] > 2 {
		return fmt.Errorf("OID: first arc must be 0, 1, or 2, got %d", oid.Arcs[0])
	}

	// Validate second arc constraints based on first arc
	if oid.Arcs[0] < 2 && oid.Arcs[1] >= 40 {
		return fmt.Errorf("OID: second arc must be < 40 when first arc is %d, got %d", oid.Arcs[0], oid.Arcs[1])
	}

	// 1. Convert OID to content octets
	contentOctets := oidToBytes(oid)

	// 2. Encode length (semi-constrained, lower bound = 0)
	n := int64(len(contentOctets))
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return fmt.Errorf("OID: failed to encode length: %w", err)
	}

	// 3. Align for APER before writing octets
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// 4. Write content octets
	for _, b := range contentOctets {
		e.stream.WriteBits(uint64(b), 8)
	}

	return nil
}

// DecodeObjectIdentifier decodes an OBJECT IDENTIFIER per X.691 Clause 24.
func (d *Decoder) DecodeObjectIdentifier() (ObjectIdentifier, error) {
	// 1. Decode length (semi-constrained, lower bound = 0)
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	n, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return ObjectIdentifier{}, fmt.Errorf("OID: failed to decode length: %w", err)
	}

	if n == 0 {
		return ObjectIdentifier{}, fmt.Errorf("OID: length cannot be zero")
	}

	// 2. Align for APER before reading octets
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return ObjectIdentifier{}, fmt.Errorf("OID: failed to align: %w", err)
			}
		}
	}

	// 3. Read content octets
	contentOctets := make([]byte, n)
	for i := range n {
		bits, err := d.stream.ReadBits(8)
		if err != nil {
			return ObjectIdentifier{}, fmt.Errorf("OID: failed to read octet at %d: %w", i, err)
		}
		contentOctets[i] = byte(bits)
	}

	// 4. Convert content octets back to OID
	oid, err := bytesToOID(contentOctets)
	if err != nil {
		return ObjectIdentifier{}, fmt.Errorf("OID: failed to parse content: %w", err)
	}

	return oid, nil
}

// oidToBytes converts an OID to BER content octets.
// Per X.690 Clause 8.19:
// - First octet = first_arc * 40 + second_arc
// - Subsequent arcs encoded in base-128 (VLQ: Variable Length Quantity)
func oidToBytes(oid ObjectIdentifier) []byte {
	var result []byte

	// First octet: combine first two arcs
	firstOctet := byte(oid.Arcs[0]*40 + oid.Arcs[1])
	result = append(result, firstOctet)

	// Remaining arcs: encode each in base-128
	for i := 2; i < len(oid.Arcs); i++ {
		arcBytes := encodeBase128(oid.Arcs[i])
		result = append(result, arcBytes...)
	}

	return result
}

// bytesToOID converts BER content octets to an OID.
func bytesToOID(octets []byte) (ObjectIdentifier, error) {
	if len(octets) == 0 {
		return ObjectIdentifier{}, fmt.Errorf("empty OID octets")
	}

	var arcs []uint64

	// First octet: split into first two arcs
	firstOctet := octets[0]
	if firstOctet < 40 {
		arcs = append(arcs, 0, uint64(firstOctet))
	} else if firstOctet < 80 {
		arcs = append(arcs, 1, uint64(firstOctet-40))
	} else {
		arcs = append(arcs, 2, uint64(firstOctet-80))
	}

	// Remaining octets: decode base-128
	i := 1
	for i < len(octets) {
		arc, bytesRead, err := decodeBase128(octets[i:])
		if err != nil {
			return ObjectIdentifier{}, err
		}
		arcs = append(arcs, arc)
		i += bytesRead
	}

	return ObjectIdentifier{Arcs: arcs}, nil
}

// encodeBase128 encodes a uint64 value in base-128 format (VLQ).
// Each byte has the high bit set (0x80) except the last byte.
func encodeBase128(value uint64) []byte {
	if value == 0 {
		return []byte{0}
	}

	// Calculate number of 7-bit groups needed
	var result []byte
	for value > 0 {
		result = append([]byte{byte(value & 0x7F)}, result...)
		value >>= 7
	}

	// Set high bit on all but the last byte
	for i := 0; i < len(result)-1; i++ {
		result[i] |= 0x80
	}

	return result
}

// decodeBase128 decodes a base-128 encoded value (VLQ).
// Returns the decoded value, number of bytes consumed, and any error.
func decodeBase128(octets []byte) (uint64, int, error) {
	if len(octets) == 0 {
		return 0, 0, fmt.Errorf("empty base-128 input")
	}

	var value uint64
	bytesRead := 0

	for bytesRead < len(octets) {
		b := octets[bytesRead]
		bytesRead++

		// Check for overflow (max ~9 bytes for uint64)
		if bytesRead > 10 {
			return 0, 0, fmt.Errorf("base-128 value too large")
		}

		value = (value << 7) | uint64(b&0x7F)

		// If high bit is not set, this is the last byte
		if b&0x80 == 0 {
			return value, bytesRead, nil
		}
	}

	return 0, 0, fmt.Errorf("incomplete base-128 value")
}

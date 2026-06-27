package per

import (
	"fmt"
	"strings"
)

// RelativeObjectIdentifier represents an ASN.1 RELATIVE-OID value.
// Unlike OID, RELATIVE-OID does not combine the first two arcs.
// X.691 Clause 24: Encoded similarly to OID but all arcs use base-128.
type RelativeObjectIdentifier struct {
	Arcs []uint64 // Component arcs (e.g., [8571, 3, 2] for "8571.3.2")
}

// NewRelativeObjectIdentifier creates a new RelativeObjectIdentifier from arc values.
func NewRelativeObjectIdentifier(arcs ...uint64) RelativeObjectIdentifier {
	return RelativeObjectIdentifier{Arcs: arcs}
}

// String returns the dot-notation string representation (e.g., "8571.3.2").
func (roid RelativeObjectIdentifier) String() string {
	if len(roid.Arcs) == 0 {
		return ""
	}

	parts := make([]string, len(roid.Arcs))
	for i, arc := range roid.Arcs {
		parts[i] = fmt.Sprintf("%d", arc)
	}
	return strings.Join(parts, ".")
}

// Equal returns true if this RELATIVE-OID equals another.
func (roid RelativeObjectIdentifier) Equal(other RelativeObjectIdentifier) bool {
	if len(roid.Arcs) != len(other.Arcs) {
		return false
	}
	for i := range roid.Arcs {
		if roid.Arcs[i] != other.Arcs[i] {
			return false
		}
	}
	return true
}

// EncodeRelativeObjectIdentifier encodes a RELATIVE-OID per X.691 Clause 24.
//
// PER encodes RELATIVE-OID by:
// 1. Converting all arcs to base-128 (VLQ) encoding
// 2. Encoding length as semi-constrained (lower bound = 0)
// 3. Writing content octets (with APER alignment before octets)
func (e *Encoder) EncodeRelativeObjectIdentifier(roid RelativeObjectIdentifier) error {
	if len(roid.Arcs) == 0 {
		return fmt.Errorf("RELATIVE-OID: must have at least 1 arc")
	}

	// 1. Convert RELATIVE-OID to content octets (all arcs use base-128)
	contentOctets := relOIDToBytes(roid)

	// 2. Encode length (semi-constrained, lower bound = 0)
	n := int64(len(contentOctets))
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return fmt.Errorf("RELATIVE-OID: failed to encode length: %w", err)
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

// DecodeRelativeObjectIdentifier decodes a RELATIVE-OID per X.691 Clause 24.
func (d *Decoder) DecodeRelativeObjectIdentifier() (RelativeObjectIdentifier, error) {
	// 1. Decode length (semi-constrained, lower bound = 0)
	constraints := SizeConstraints{
		Min:        nil, // Semi-constrained
		Max:        nil,
		Extensible: false,
	}
	n, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return RelativeObjectIdentifier{}, fmt.Errorf("RELATIVE-OID: failed to decode length: %w", err)
	}

	if n == 0 {
		return RelativeObjectIdentifier{}, fmt.Errorf("RELATIVE-OID: length cannot be zero")
	}

	// 2. Align for APER before reading octets
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return RelativeObjectIdentifier{}, fmt.Errorf("RELATIVE-OID: failed to align: %w", err)
			}
		}
	}

	// 3. Read content octets
	contentOctets := make([]byte, n)
	for i := range n {
		bits, err := d.stream.ReadBits(8)
		if err != nil {
			return RelativeObjectIdentifier{}, fmt.Errorf("RELATIVE-OID: failed to read octet at %d: %w", i, err)
		}
		contentOctets[i] = byte(bits)
	}

	// 4. Convert content octets back to RELATIVE-OID
	roid, err := bytesToRelOID(contentOctets)
	if err != nil {
		return RelativeObjectIdentifier{}, fmt.Errorf("RELATIVE-OID: failed to parse content: %w", err)
	}

	return roid, nil
}

// relOIDToBytes converts a RELATIVE-OID to content octets.
// All arcs are encoded in base-128 (VLQ).
func relOIDToBytes(roid RelativeObjectIdentifier) []byte {
	var result []byte

	// Encode each arc in base-128
	for _, arc := range roid.Arcs {
		arcBytes := encodeBase128(arc)
		result = append(result, arcBytes...)
	}

	return result
}

// bytesToRelOID converts content octets to a RELATIVE-OID.
func bytesToRelOID(octets []byte) (RelativeObjectIdentifier, error) {
	if len(octets) == 0 {
		return RelativeObjectIdentifier{}, fmt.Errorf("empty RELATIVE-OID octets")
	}

	var arcs []uint64

	// Decode all base-128 arcs
	i := 0
	for i < len(octets) {
		arc, bytesRead, err := decodeBase128(octets[i:])
		if err != nil {
			return RelativeObjectIdentifier{}, err
		}
		arcs = append(arcs, arc)
		i += bytesRead
	}

	return RelativeObjectIdentifier{Arcs: arcs}, nil
}

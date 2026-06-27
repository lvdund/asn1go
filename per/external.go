package per

// EXTERNAL type implementation per X.691 Clause 29.
//
// EXTERNAL is defined in X.680 as:
//   [UNIVERSAL 8] IMPLICIT SEQUENCE {
//     direct-reference OBJECT IDENTIFIER OPTIONAL,
//     indirect-reference INTEGER OPTIONAL,
//     data-value-descriptor ObjectDescriptor OPTIONAL,
//     encoding CHOICE {
//       single-ASN1-type [0] ABSTRACT-SYNTAX.&Type,
//       octet-aligned [1] IMPLICIT OCTET STRING,
//       arbitrary [2] IMPLICIT BIT STRING
//     }
//   }
//
// Note: EXTERNAL is a complex type that is rarely used in modern protocols.
// It requires full SEQUENCE, CHOICE, OPTIONAL field, and OBJECT IDENTIFIER support.
//
// This implementation provides the framework for EXTERNAL encoding/decoding,
// but users should encode/decode EXTERNAL as a SEQUENCE using the
// existing SequenceEncoder/SequenceDecoder with appropriate component encoders.
//
// Example usage:
//   1. Create SequenceEncoder with EXTERNAL's component constraints
//   2. Encode optional OID (if present)
//   3. Encode optional INTEGER (if present)
//   4. Encode CHOICE for encoding field
//
// See X.691 Clause 29 for complete encoding rules.

// ExternalEncoding represents the encoding choice in EXTERNAL.
type ExternalEncoding int

const (
	ExternalSingleASN1Type ExternalEncoding = 0
	ExternalOctetAligned   ExternalEncoding = 1
	ExternalArbitrary      ExternalEncoding = 2
)

// External represents an ASN.1 EXTERNAL value.
// This is a simplified representation for documentation purposes.
type External struct {
	DirectReference     *ObjectIdentifier // OPTIONAL
	IndirectReference   *int64            // OPTIONAL
	DataValueDescriptor *string           // OPTIONAL (ObjectDescriptor)
	Encoding            ExternalEncoding  // CHOICE tag
	EncodingValue       []byte            // Encoded value for the chosen encoding
}

// EncodeExternal encodes an EXTERNAL value.
//
// Note: This is a placeholder. In practice, EXTERNAL should be encoded
// using SequenceEncoder with proper component handling.
func (e *Encoder) EncodeExternal(ext External) error {
	// EXTERNAL is encoded as a SEQUENCE with optional fields and a CHOICE.
	// Implementation would use:
	// 1. SequenceEncoder for the outer SEQUENCE
	// 2. Component encoders for OID, INTEGER, ObjectDescriptor
	// 3. ChoiceEncoder for the encoding CHOICE
	//
	// For now, return an error indicating this is not yet fully implemented.
	return &UnsupportedTypeError{TypeName: "EXTERNAL", Message: "Use SequenceEncoder with component encoders"}
}

// DecodeExternal decodes an EXTERNAL value.
//
// Note: This is a placeholder. In practice, EXTERNAL should be decoded
// using SequenceDecoder with proper component handling.
func (d *Decoder) DecodeExternal() (External, error) {
	// EXTERNAL is decoded as a SEQUENCE with optional fields and a CHOICE.
	// Implementation would use:
	// 1. SequenceDecoder for the outer SEQUENCE
	// 2. Component decoders for OID, INTEGER, ObjectDescriptor
	// 3. ChoiceDecoder for the encoding CHOICE
	//
	// For now, return an error indicating this is not yet fully implemented.
	return External{}, &UnsupportedTypeError{TypeName: "EXTERNAL", Message: "Use SequenceDecoder with component decoders"}
}

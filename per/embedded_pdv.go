package per

// EMBEDDED PDV type implementation per X.691 Clause 30.
//
// EMBEDDED PDV (Presentation Data Value) is defined in X.680 as:
//   [UNIVERSAL 11] IMPLICIT SEQUENCE {
//     identification CHOICE {
//       syntaxes SEQUENCE {
//         abstract OBJECT IDENTIFIER,
//         transfer OBJECT IDENTIFIER
//       },
//       syntax OBJECT IDENTIFIER,
//       presentation-context-id INTEGER,
//       context-negotiation SEQUENCE {
//         presentation-context-id INTEGER,
//         transfer-syntax OBJECT IDENTIFIER
//       },
//       transfer-syntax OBJECT IDENTIFIER,
//       fixed NULL
//     },
//     data-value-descriptor ObjectDescriptor OPTIONAL, -- removed in X.691
//     data-value OCTET STRING
//   }
//
// Note: EMBEDDED PDV is a complex type rarely used in modern protocols.
// It requires full SEQUENCE, CHOICE, OPTIONAL field, and OBJECT IDENTIFIER support.
//
// This implementation provides the framework for EMBEDDED PDV encoding/decoding,
// but users should encode/decode EMBEDDED PDV as a SEQUENCE using the
// existing SequenceEncoder/SequenceDecoder with appropriate component encoders.
//
// See X.691 Clause 30 for complete encoding rules.

// EmbeddedPDVIdentification represents the identification choice.
type EmbeddedPDVIdentification int

const (
	EmbeddedPDVSyntaxes              EmbeddedPDVIdentification = 0
	EmbeddedPDVSyntax                EmbeddedPDVIdentification = 1
	EmbeddedPDVPresentationContextID EmbeddedPDVIdentification = 2
	EmbeddedPDVContextNegotiation    EmbeddedPDVIdentification = 3
	EmbeddedPDVTransferSyntax        EmbeddedPDVIdentification = 4
	EmbeddedPDVFixed                 EmbeddedPDVIdentification = 5
)

// EmbeddedPDV represents an ASN.1 EMBEDDED PDV value.
// This is a simplified representation for documentation purposes.
type EmbeddedPDV struct {
	Identification      EmbeddedPDVIdentification
	IdentificationValue interface{} // Type depends on Identification choice
	DataValue           []byte      // OCTET STRING
}

// EncodeEmbeddedPDV encodes an EMBEDDED PDV value.
//
// Note: This is a placeholder. In practice, EMBEDDED PDV should be encoded
// using SequenceEncoder with proper component handling.
func (e *Encoder) EncodeEmbeddedPDV(pdv EmbeddedPDV) error {
	// EMBEDDED PDV is encoded as a SEQUENCE with a CHOICE and OCTET STRING.
	// Implementation would use:
	// 1. SequenceEncoder for the outer SEQUENCE
	// 2. ChoiceEncoder for the identification CHOICE
	// 3. OctetString encoder for data-value
	//
	// For now, return an error indicating this is not yet fully implemented.
	return &UnsupportedTypeError{TypeName: "EMBEDDED PDV", Message: "Use SequenceEncoder with component encoders"}
}

// DecodeEmbeddedPDV decodes an EMBEDDED PDV value.
//
// Note: This is a placeholder. In practice, EMBEDDED PDV should be decoded
// using SequenceDecoder with proper component handling.
func (d *Decoder) DecodeEmbeddedPDV() (EmbeddedPDV, error) {
	// EMBEDDED PDV is decoded as a SEQUENCE with a CHOICE and OCTET STRING.
	// Implementation would use:
	// 1. SequenceDecoder for the outer SEQUENCE
	// 2. ChoiceDecoder for the identification CHOICE
	// 3. OctetString decoder for data-value
	//
	// For now, return an error indicating this is not yet fully implemented.
	return EmbeddedPDV{}, &UnsupportedTypeError{TypeName: "EMBEDDED PDV", Message: "Use SequenceDecoder with component decoders"}
}

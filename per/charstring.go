package per

import (
	"fmt"
	"math"
	"strings"
)

// Standard character set alphabets per ITU-T X.680
var standardAlphabets = map[string]string{
	"NumericString":   "0123456789 ",
	"PrintableString": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 '()+,-./:=?",
	"IA5String":       "", // ASCII 0-127 (128 chars)
	"VisibleString":   "", // ASCII 32-126 (95 chars)
	"BMPString":       "", // Unicode BMP (65536 chars)
	"UniversalString": "", // Unicode (4 bytes per char)
	"UTF8String":      "", // UTF-8 encoding
}

// CharacterStringConstraints defines constraints for restricted character strings.
type CharacterStringConstraints struct {
	PermittedAlphabet string          // Permitted alphabet (if constrained)
	SizeConstraints   SizeConstraints // Size constraints
	TypeName          string          // String type name (for standard alphabets)
}

// EncodeRestrictedString encodes a restricted character string per X.691 Clause 31.
func (e *Encoder) EncodeRestrictedString(value string, constraints CharacterStringConstraints) error {
	// Determine effective alphabet
	alphabet := constraints.PermittedAlphabet
	if alphabet == "" {
		alphabet = getStandardAlphabet(constraints.TypeName)
	}

	// Calculate bits per character
	bitsPerChar := calculateBitsPerChar(len(alphabet), e.variant)
	indexBased := isIndexBasedAlphabet(constraints)

	// Check extensibility
	if constraints.SizeConstraints.Extensible {
		inRoot := false
		n := int64(len(value))
		if constraints.SizeConstraints.Min != nil && constraints.SizeConstraints.Max != nil {
			inRoot = n >= *constraints.SizeConstraints.Min && n <= *constraints.SizeConstraints.Max
		}

		if inRoot {
			// Extension bit: 0 = in root
			e.stream.WriteBit(0)
			return e.encodeRootString(value, alphabet, bitsPerChar, constraints.SizeConstraints, indexBased)
		} else {
			// Extension bit: 1 = in extension
			e.stream.WriteBit(1)
			return e.encodeExtensionString(value, alphabet, bitsPerChar, indexBased)
		}
	}

	// No extension marker
	return e.encodeRootString(value, alphabet, bitsPerChar, constraints.SizeConstraints, indexBased)
}

// encodeRootString handles root string encoding.
func (e *Encoder) encodeRootString(value string, alphabet string, bitsPerChar int, sizeConstraints SizeConstraints, indexBased bool) error {
	n := int64(len(value))
	min := sizeConstraints.Min
	max := sizeConstraints.Max

	// Fixed length
	if max != nil && min != nil && *max == *min {
		if n != *max {
			return &ConstraintViolationError{
				TypeName:   "RestrictedString",
				Value:      n,
				Constraint: "length must match fixed size",
				Position:   e.stream.Position(),
			}
		}

		totalBits := int(*max) * bitsPerChar

		// APER: Align if total bits > 16
		if e.variant == APER && totalBits > 16 {
			e.stream.AlignToOctet()
		}

		// Encode characters
		for _, char := range value {
			val, err := charToCode(char, alphabet, indexBased)
			if err != nil {
				return &ConstraintViolationError{
					TypeName:   "RestrictedString",
					Value:      char,
					Constraint: "character not in permitted alphabet",
					Position:   e.stream.Position(),
				}
			}
			e.stream.WriteBits(uint64(val), bitsPerChar)
		}
		return nil
	}

	// Variable length: encode length determinant
	if err := e.EncodeLengthDeterminant(n, sizeConstraints); err != nil {
		return err
	}

	// APER: Align if needed
	if e.variant == APER {
		// Align if unconstrained or if ub * B >= 16
		if max == nil || (int(*max)*bitsPerChar) >= 16 {
			e.stream.AlignToOctet()
		}
	}

	// Encode characters
	for _, char := range value {
		val, err := charToCode(char, alphabet, indexBased)
		if err != nil {
			return &ConstraintViolationError{
				TypeName:   "RestrictedString",
				Value:      char,
				Constraint: "character not in permitted alphabet",
				Position:   e.stream.Position(),
			}
		}
		e.stream.WriteBits(uint64(val), bitsPerChar)
	}

	return nil
}

// encodeExtensionString handles extension string encoding.
func (e *Encoder) encodeExtensionString(value string, alphabet string, bitsPerChar int, indexBased bool) error {
	n := int64(len(value))

	// Unconstrained length
	constraints := SizeConstraints{Min: nil, Max: nil}
	if err := e.EncodeLengthDeterminant(n, constraints); err != nil {
		return err
	}

	// APER: Align
	if e.variant == APER {
		e.stream.AlignToOctet()
	}

	// Encode characters
	for _, char := range value {
		val, err := charToCode(char, alphabet, indexBased)
		if err != nil {
			return &ConstraintViolationError{
				TypeName:   "RestrictedString",
				Value:      char,
				Constraint: "character not in permitted alphabet",
				Position:   e.stream.Position(),
			}
		}
		e.stream.WriteBits(uint64(val), bitsPerChar)
	}

	return nil
}

// DecodeRestrictedString decodes a restricted character string per X.691 Clause 31.
func (d *Decoder) DecodeRestrictedString(constraints CharacterStringConstraints) (string, error) {
	// Determine effective alphabet
	alphabet := constraints.PermittedAlphabet
	if alphabet == "" {
		alphabet = getStandardAlphabet(constraints.TypeName)
	}

	// Calculate bits per character
	bitsPerChar := calculateBitsPerChar(len(alphabet), d.variant)
	indexBased := isIndexBasedAlphabet(constraints)

	// Check extensibility
	if constraints.SizeConstraints.Extensible {
		// Read extension bit
		extBit, err := d.stream.ReadBit()
		if err != nil {
			return "", &DecodeError{
				TypeName: "RestrictedString",
				Reason:   "failed to read extension bit",
				Position: d.stream.Position(),
			}
		}

		if extBit == 1 {
			// In extension
			return d.decodeExtensionString(alphabet, bitsPerChar, indexBased)
		}
	}

	// In root
	return d.decodeRootString(alphabet, bitsPerChar, constraints.SizeConstraints, indexBased)
}

// decodeRootString handles root string decoding.
func (d *Decoder) decodeRootString(alphabet string, bitsPerChar int, sizeConstraints SizeConstraints, indexBased bool) (string, error) {
	min := sizeConstraints.Min
	max := sizeConstraints.Max

	var length int64

	// Fixed length
	if max != nil && min != nil && *max == *min {
		length = *max

		totalBits := int(*max) * bitsPerChar

		// APER: Read alignment if total bits > 16
		if d.variant == APER && totalBits > 16 {
			for d.stream.Position()%8 != 0 {
				_, err := d.stream.ReadBit()
				if err != nil {
					return "", err
				}
			}
		}
	} else {
		// Variable length: decode length determinant
		var err error
		length, err = d.DecodeLengthDeterminant(sizeConstraints)
		if err != nil {
			return "", err
		}

		// APER: Read alignment if needed
		if d.variant == APER {
			if max == nil || (int(*max)*bitsPerChar) >= 16 {
				for d.stream.Position()%8 != 0 {
					_, err := d.stream.ReadBit()
					if err != nil {
						return "", err
					}
				}
			}
		}
	}

	// Decode characters
	var result strings.Builder
	for i := int64(0); i < length; i++ {
		val, err := d.stream.ReadBits(bitsPerChar)
		if err != nil {
			return "", &DecodeError{
				TypeName: "RestrictedString",
				Reason:   "failed to read character bits",
				Position: d.stream.Position(),
			}
		}

		char, err := codeToChar(int(val), alphabet, indexBased)
		if err != nil {
			return "", &DecodeError{
				TypeName: "RestrictedString",
				Reason:   fmt.Sprintf("invalid character value: %d", val),
				Position: d.stream.Position(),
			}
		}

		result.WriteRune(char)
	}

	return result.String(), nil
}

// decodeExtensionString handles extension string decoding.
func (d *Decoder) decodeExtensionString(alphabet string, bitsPerChar int, indexBased bool) (string, error) {
	// Unconstrained length
	constraints := SizeConstraints{Min: nil, Max: nil}
	length, err := d.DecodeLengthDeterminant(constraints)
	if err != nil {
		return "", err
	}

	// APER: Read alignment
	if d.variant == APER {
		for d.stream.Position()%8 != 0 {
			_, err := d.stream.ReadBit()
			if err != nil {
				return "", err
			}
		}
	}

	// Decode characters
	var result strings.Builder
	for range length {
		val, err := d.stream.ReadBits(bitsPerChar)
		if err != nil {
			return "", &DecodeError{
				TypeName: "RestrictedString",
				Reason:   "failed to read character bits",
				Position: d.stream.Position(),
			}
		}

		char, err := codeToChar(int(val), alphabet, indexBased)
		if err != nil {
			return "", &DecodeError{
				TypeName: "RestrictedString",
				Reason:   fmt.Sprintf("invalid character value: %d", val),
				Position: d.stream.Position(),
			}
		}

		result.WriteRune(char)
	}

	return result.String(), nil
}

// Helper functions

// getStandardAlphabet returns the standard alphabet for a type name.
func getStandardAlphabet(typeName string) string {
	if alpha, ok := standardAlphabets[typeName]; ok {
		if alpha == "" {
			// Special handling for standard types with large alphabets
			switch typeName {
			case "IA5String":
				// ASCII 0-127
				var sb strings.Builder
				for i := range 128 {
					sb.WriteByte(byte(i))
				}
				return sb.String()
			case "VisibleString":
				// ASCII 32-126
				var sb strings.Builder
				for i := 32; i <= 126; i++ {
					sb.WriteByte(byte(i))
				}
				return sb.String()
			}
		}
		return alpha
	}
	// Default: unrestricted (full byte range)
	return ""
}

// calculateBitsPerChar calculates bits per character based on alphabet size.
func calculateBitsPerChar(alphabetSize int, variant Variant) int {
	if alphabetSize <= 1 {
		return 0
	}

	// Calculate minimum bits needed
	bitsMin := int(math.Ceil(math.Log2(float64(alphabetSize))))

	if variant == APER {
		// APER: Round up to next power of 2 (1, 2, 4, 8, 16, 32...)
		if bitsMin <= 1 {
			return 1
		}
		if bitsMin <= 2 {
			return 2
		}
		if bitsMin <= 4 {
			return 4
		}
		if bitsMin <= 8 {
			return 8
		}
		if bitsMin <= 16 {
			return 16
		}
		return 32
	}

	// UPER: Exact bits needed
	return bitsMin
}

// isIndexBasedAlphabet reports whether characters are encoded as their alphabet
// INDEX (rather than their code point). Only NumericString and explicit
// user-supplied permitted alphabets use index-based mapping; the standard
// known-multiplier types (IA5String, VisibleString, PrintableString, BMPString,
// ...) use an identity map {code: code} and encode the raw character code, per
// asn1tools' ENCODE_DECODE_MAP.
func isIndexBasedAlphabet(constraints CharacterStringConstraints) bool {
	return constraints.TypeName == "NumericString" || constraints.PermittedAlphabet != ""
}

// charToCode maps a character to its PER encoding value.
func charToCode(char rune, alphabet string, indexBased bool) (int, error) {
	if indexBased {
		idx := strings.IndexRune(alphabet, char)
		if idx < 0 {
			return 0, fmt.Errorf("character '%c' not in alphabet", char)
		}
		return idx, nil
	}
	if !strings.ContainsRune(alphabet, char) {
		return 0, fmt.Errorf("character '%c' not in alphabet", char)
	}
	return int(char), nil
}

// codeToChar maps a PER encoding value back to a character.
func codeToChar(value int, alphabet string, indexBased bool) (rune, error) {
	if indexBased {
		if value < 0 || value >= len(alphabet) {
			return 0, fmt.Errorf("value %d out of alphabet range", value)
		}
		return rune(alphabet[value]), nil
	}
	char := rune(value)
	if !strings.ContainsRune(alphabet, char) {
		return 0, fmt.Errorf("value %d not in alphabet", value)
	}
	return char, nil
}

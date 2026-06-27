package per

import "fmt"

// ConstraintViolationError indicates that a value violates its PER-visible constraint.
// X.691 requires strict constraint enforcement during encoding.
type ConstraintViolationError struct {
	TypeName   string      // ASN.1 type name (e.g., "INTEGER", "OCTET STRING")
	Value      interface{} // The value that violated the constraint
	Constraint string      // Human-readable constraint description
	Position   int         // Bit position in stream when error occurred
}

// Error returns a formatted error message including all context.
func (e *ConstraintViolationError) Error() string {
	return fmt.Sprintf("%s: value %v violates constraint %s at bit %d",
		e.TypeName, e.Value, e.Constraint, e.Position)
}

// DecodeError indicates a problem during PER decoding.
// This can occur due to malformed input, truncated streams, or invalid encodings.
type DecodeError struct {
	TypeName string // ASN.1 type being decoded
	Reason   string // Description of the problem
	Position int    // Bit position where error occurred
	Expected int    // Expected number of bits (for truncation errors)
	Got      int    // Actual number of bits available
}

// Error returns a formatted error message with decode context.
func (e *DecodeError) Error() string {
	if e.Expected > 0 {
		return fmt.Sprintf("%s: %s at bit %d (expected %d more bits, got %d)",
			e.TypeName, e.Reason, e.Position, e.Expected, e.Got)
	}
	return fmt.Sprintf("%s: %s at bit %d", e.TypeName, e.Reason, e.Position)
}

// InvalidVariantError indicates that an invalid variant was used or
// that an operation is not supported for the given variant.
type InvalidVariantError struct {
	Operation string  // Operation being attempted
	Variant   Variant // The variant that was invalid
}

// Error returns a formatted error message.
func (e *InvalidVariantError) Error() string {
	return fmt.Sprintf("invalid variant %s for operation: %s", e.Variant, e.Operation)
}

// UnsupportedTypeError indicates that a type is not yet fully implemented
// or requires composition of other encoders/decoders.
type UnsupportedTypeError struct {
	TypeName string // ASN.1 type name
	Message  string // Additional context or guidance
}

// Error returns a formatted error message.
func (e *UnsupportedTypeError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: not fully implemented - %s", e.TypeName, e.Message)
	}
	return fmt.Sprintf("%s: not fully implemented", e.TypeName)
}

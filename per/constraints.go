package per

import "math/big"

// IntegerConstraints defines PER-visible constraints for INTEGER type.
// X.691 Clauses 13-14 specify different encoding based on constraint type.
type IntegerConstraints struct {
	Extensible bool   // Has extension marker (...)
	LowerBound *int64 // Lower bound (nil = no lower bound)
	UpperBound *int64 // Upper bound (nil = unconstrained/semi-constrained)

	// BigIntLower/BigIntUpper optionally specify bounds that exceed the int64
	// range (e.g. INTEGER (0..18446744073709551615)). When non-nil they take
	// precedence over LowerBound/UpperBound and are used by EncodeBigInteger /
	// DecodeBigInteger. The int64-based EncodeInteger/DecodeInteger ignore them.
	BigIntLower *big.Int
	BigIntUpper *big.Int
}

// Unconstrained returns constraints for an unconstrained INTEGER.
// Encoded as 2's complement with length determinant.
func Unconstrained() IntegerConstraints {
	return IntegerConstraints{
		Extensible: false,
		LowerBound: nil,
		UpperBound: nil,
	}
}

// Constrained returns constraints for INTEGER(lb..ub).
// Uses minimum bits for range when R <= 64K, otherwise length determinant.
func Constrained(lb, ub int64) IntegerConstraints {
	return IntegerConstraints{
		Extensible: false,
		LowerBound: &lb,
		UpperBound: &ub,
	}
}

// SemiConstrained returns constraints for INTEGER(lb..MAX).
// Encoded as length determinant + offset from lower bound.
func SemiConstrained(lb int64) IntegerConstraints {
	return IntegerConstraints{
		Extensible: false,
		LowerBound: &lb,
		UpperBound: nil,
	}
}

// ConstrainedBig returns constraints for INTEGER(lb..ub) whose bounds may
// exceed the int64 range. Use with EncodeBigInteger / DecodeBigInteger.
func ConstrainedBig(lb, ub *big.Int) IntegerConstraints {
	return IntegerConstraints{
		Extensible:  false,
		BigIntLower: new(big.Int).Set(lb),
		BigIntUpper: new(big.Int).Set(ub),
	}
}

// ConstrainedExtensible returns constraints for INTEGER(lb..ub, ...).
// Values outside root range encode with extension bit set.
func ConstrainedExtensible(lb, ub int64) IntegerConstraints {
	return IntegerConstraints{
		Extensible: true,
		LowerBound: &lb,
		UpperBound: &ub,
	}
}

// SizeConstraints defines PER-visible SIZE constraints.
// Applies to BIT STRING, OCTET STRING, character strings, SEQUENCE OF, SET OF.
type SizeConstraints struct {
	Extensible bool   // Has extension marker
	Min        *int64 // Minimum size (nil = 0)
	Max        *int64 // Maximum size (nil = unbounded)
}

// FixedSize returns SIZE(n) constraint.
// No length determinant needed (fixed size known from constraint).
func FixedSize(n int64) SizeConstraints {
	size := n
	return SizeConstraints{
		Extensible: false,
		Min:        &size,
		Max:        &size,
	}
}

// SizeRange returns SIZE(min..max) constraint.
// Length determinant uses constrained whole number encoding.
func SizeRange(min, max int64) SizeConstraints {
	minVal := min
	maxVal := max
	return SizeConstraints{
		Extensible: false,
		Min:        &minVal,
		Max:        &maxVal,
	}
}

// SizeMin returns SIZE(min..MAX) constraint.
// Length determinant uses normally small length encoding.
func SizeMin(min int64) SizeConstraints {
	minVal := min
	return SizeConstraints{
		Extensible: false,
		Min:        &minVal,
		Max:        nil,
	}
}

// EnumeratedConstraints defines ENUMERATED type constraints.
// X.691 Clause 14: encodes as constrained integer (index).
type EnumeratedConstraints struct {
	Extensible bool    // Has extension marker
	RootValues []int64 // Root enumeration values (sorted by value)
	ExtValues  []int64 // Extension enumeration values (sorted by value)
}

// ComponentInfo describes a single SEQUENCE/SET component.
// X.691 Clause 19 (SEQUENCE) and Clause 21 (SET): affects preamble bitmap encoding.
type ComponentInfo struct {
	Name       string      // Component name (for debugging)
	Tag        int         // Component tag number (for canonical ordering in SET)
	Optional   bool        // OPTIONAL marker present
	HasDefault bool        // DEFAULT marker present
	Default    interface{} // Default value (if HasDefault is true)
}

// SequenceConstraints defines SEQUENCE component information.
// X.691 Clause 19: determines preamble and extension encoding.
type SequenceConstraints struct {
	Extensible     bool            // Has extension marker
	RootComponents []ComponentInfo // Root components in definition order
	ExtComponents  []ComponentInfo // Extension components
}

// AlternativeInfo describes a single CHOICE alternative.
// X.691 Clause 23: affects choice index encoding.
type AlternativeInfo struct {
	Name string // Alternative name (for debugging)
	Tag  int    // Tag number for canonical ordering
}

// ChoiceConstraints defines CHOICE alternative information.
// X.691 Clause 23: determines index encoding width.
type ChoiceConstraints struct {
	Extensible       bool              // Has extension marker
	RootAlternatives []AlternativeInfo // Root alternatives
	ExtAlternatives  []AlternativeInfo // Extension alternatives
}

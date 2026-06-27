// Package per implements ASN.1 Packed Encoding Rules (PER) per ITU-T X.691.
//
// This package supports both Aligned PER (APER) and Unaligned PER (UPER) variants.
// APER aligns certain fields to octet boundaries for easier processing but larger size.
// UPER uses minimum bits without alignment for smaller encodings.
package per

// Variant specifies the PER encoding variant.
// X.691 defines two variants with different alignment rules.
type Variant int

const (
	// APER (Aligned PER) aligns to octet boundaries at specific points per X.691.
	// Used when byte-aligned access is beneficial or required.
	APER Variant = iota

	// UPER (Unaligned PER) uses strict bit-level encoding without padding.
	// Produces smallest encodings but requires bit-level processing.
	UPER
)

// String returns the variant name for debugging and logging.
func (v Variant) String() string {
	switch v {
	case APER:
		return "APER"
	case UPER:
		return "UPER"
	default:
		return "UNKNOWN"
	}
}

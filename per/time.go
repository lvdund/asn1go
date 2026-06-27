package per

import (
	"fmt"
	"time"
)

// EncodeGeneralizedTime encodes an ASN.1 GeneralizedTime value.
//
// GeneralizedTime is encoded as a VisibleString with format:
// "YYYYMMDDHHMMSSZ" (basic format, UTC, no fractional seconds)
//
// X.691 Clause 28: GeneralizedTime is encoded as a restricted character string.
func (e *Encoder) EncodeGeneralizedTime(t time.Time) error {
	// Convert to UTC and format as "YYYYMMDDHHMMSSZ"
	utc := t.UTC()
	str := utc.Format("20060102150405Z")

	// Encode as VisibleString (unconstrained)
	constraints := CharacterStringConstraints{
		TypeName:          "VisibleString",
		PermittedAlphabet: "",
		SizeConstraints: SizeConstraints{
			Min:        nil,
			Max:        nil,
			Extensible: false,
		},
	}

	if err := e.EncodeRestrictedString(str, constraints); err != nil {
		return fmt.Errorf("GeneralizedTime: %w", err)
	}

	return nil
}

// DecodeGeneralizedTime decodes an ASN.1 GeneralizedTime value.
func (d *Decoder) DecodeGeneralizedTime() (time.Time, error) {
	// Decode as VisibleString (unconstrained)
	constraints := CharacterStringConstraints{
		TypeName:          "VisibleString",
		PermittedAlphabet: "",
		SizeConstraints: SizeConstraints{
			Min:        nil,
			Max:        nil,
			Extensible: false,
		},
	}

	str, err := d.DecodeRestrictedString(constraints)
	if err != nil {
		return time.Time{}, fmt.Errorf("GeneralizedTime: %w", err)
	}

	// Parse "YYYYMMDDHHMMSSZ" format
	t, err := time.Parse("20060102150405Z", str)
	if err != nil {
		return time.Time{}, fmt.Errorf("GeneralizedTime: invalid format: %w", err)
	}

	return t, nil
}

// EncodeUTCTime encodes an ASN.1 UTCTime value.
//
// UTCTime is encoded as a VisibleString with format:
// "YYMMDDHHMMSSZ" (2-digit year, basic format, UTC)
//
// X.691 Clause 27: UTCTime is encoded as a restricted character string.
// Years 00-49 → 2000-2049
// Years 50-99 → 1950-1999
func (e *Encoder) EncodeUTCTime(t time.Time) error {
	// Convert to UTC and format as "YYMMDDHHMMSSZ"
	utc := t.UTC()
	str := utc.Format("060102150405Z")

	// Encode as VisibleString (unconstrained)
	constraints := CharacterStringConstraints{
		TypeName:          "VisibleString",
		PermittedAlphabet: "",
		SizeConstraints: SizeConstraints{
			Min:        nil,
			Max:        nil,
			Extensible: false,
		},
	}

	if err := e.EncodeRestrictedString(str, constraints); err != nil {
		return fmt.Errorf("UTCTime: %w", err)
	}

	return nil
}

// DecodeUTCTime decodes an ASN.1 UTCTime value.
func (d *Decoder) DecodeUTCTime() (time.Time, error) {
	// Decode as VisibleString (unconstrained)
	constraints := CharacterStringConstraints{
		TypeName:          "VisibleString",
		PermittedAlphabet: "",
		SizeConstraints: SizeConstraints{
			Min:        nil,
			Max:        nil,
			Extensible: false,
		},
	}

	str, err := d.DecodeRestrictedString(constraints)
	if err != nil {
		return time.Time{}, fmt.Errorf("UTCTime: %w", err)
	}

	// Parse "YYMMDDHHMMSSZ" format
	// Go's time.Parse handles 2-digit years: 00-68 → 2000-2068, 69-99 → 1969-1999
	t, err := time.Parse("060102150405Z", str)
	if err != nil {
		return time.Time{}, fmt.Errorf("UTCTime: invalid format: %w", err)
	}

	return t, nil
}

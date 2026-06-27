package per

// EncodeBoolean encodes a BOOLEAN value per X.691 Clause 12.
// TRUE encodes as bit 1, FALSE as bit 0.
// No alignment occurs for BOOLEAN (single bit for both APER and UPER).
func (e *Encoder) EncodeBoolean(value bool) error {
	if value {
		e.stream.WriteBit(1)
	} else {
		e.stream.WriteBit(0)
	}
	return nil
}

// DecodeBoolean decodes a BOOLEAN value per X.691 Clause 12.
// Bit 1 = TRUE, bit 0 = FALSE.
func (d *Decoder) DecodeBoolean() (bool, error) {
	bit, err := d.stream.ReadBit()
	if err != nil {
		return false, &DecodeError{
			TypeName: "BOOLEAN",
			Reason:   "failed to read boolean bit",
			Position: d.stream.Position(),
		}
	}
	return bit != 0, nil
}

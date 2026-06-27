package per

// EncodeNull encodes a NULL value per X.691 Clause 13.
// NULL is encoded with no bits (zero length encoding).
// This function exists for API completeness and explicit type handling.
func (e *Encoder) EncodeNull() error {
	// X.691 Clause 13: NULL encoding produces no bits
	// Nothing to write
	return nil
}

// DecodeNull decodes a NULL value per X.691 Clause 13.
// Since NULL produces no bits, this is a no-op that verifies the position.
// This function exists for API completeness and explicit type handling.
func (d *Decoder) DecodeNull() error {
	// X.691 Clause 13: NULL decoding reads no bits
	// Nothing to read
	return nil
}

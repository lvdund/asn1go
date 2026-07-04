package per

// Open type field encoding per X.691 §11.2.
//
// An open type is a value whose concrete type is not fixed by the static schema
// (e.g. the `value` field of XNAP-PROTOCOL-IES.&Value in a ProtocolIE-Field).
// On the wire it is an unconstrained length determinant (§11.9.3.6/7/8) followed
// by n octets of the inner value's complete encoding (§11.1).
//
// The caller produces the inner payload with a separate sub-encoder and passes
// the resulting bytes to EncodeOpenType. On decode, DecodeOpenType returns the
// raw payload octets and the caller dispatches a sub-decoder based on context
// (for example the ProtocolIE-&id field) — the open-type layer treats its
// payload as opaque.

// EncodeOpenType encodes an open type field per X.691 §11.2.
//
// payload is the complete encoding (per §11.1) of the inner value, produced by
// a separate sub-encoder of the same variant. APER octet-aligns before the
// length octet (via EncodeUnconstrainedLength); the payload octets follow,
// already aligned, so no extra alignment is performed.
//
// If payload is empty it is replaced with a single zero octet per §11.1.3.1/4,
// because the complete encoding of an outermost value is never zero octets.
func (e *Encoder) EncodeOpenType(payload []byte) error {
	if len(payload) == 0 {
		payload = []byte{0x00} // §11.1.3.1/4
	}
	if err := e.EncodeUnconstrainedLength(int64(len(payload))); err != nil {
		return err
	}
	for _, b := range payload {
		e.stream.WriteBits(uint64(b), 8)
	}
	return nil
}

// DecodeOpenType decodes an open type field per X.691 §11.2 and returns the raw
// payload octets. The caller is responsible for dispatching a sub-decoder based
// on context (e.g. the surrounding ProtocolIE-&id), because the decoder cannot
// know the concrete type until that field has been read.
func (d *Decoder) DecodeOpenType() ([]byte, error) {
	n, err := d.DecodeUnconstrainedLength()
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	for i := int64(0); i < n; i++ {
		octet, err := d.stream.ReadBits(8)
		if err != nil {
			return nil, &DecodeError{
				TypeName: "OPEN TYPE",
				Reason:   "failed to read open type octet",
				Position: d.stream.Position(),
			}
		}
		out[i] = byte(octet)
	}
	return out, nil
}

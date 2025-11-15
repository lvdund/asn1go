package uper

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"

	"github.com/lvdund/asn1go/utils"
)

type UperReader struct {
	*bitstreamReader
}

func NewReader(r io.Reader) *UperReader {
	return &UperReader{
		bitstreamReader: NewBitStreamReader(r),
	}
}

func (ur *UperReader) readBytes(nbytes uint) (output []byte, err error) {
	return ur.ReadBits(nbytes * 8)
}

func (ur *UperReader) readValue(nbits uint) (v uint64, err error) {
	defer func() {
		if err != nil {
			err = utils.WrapError("readValue", err)
		}
	}()

	if nbits > 64 {
		err = ErrOverflow
		return
	}
	var buf []byte
	if buf, err = ur.ReadBits(nbits); err != nil {
		return
	}
	vBytes := make([]byte, 8)
	copy(vBytes[:], buf)
	v = binary.BigEndian.Uint64(vBytes)
	v >>= (64 - nbits)
	return
}

// readConstraintValue for UPER (no alignment)
func (ur *UperReader) readConstraintValue(r uint64) (v uint64, err error) {
	defer func() {
		err = utils.WrapError("readConstraintValue", err)
	}()

	if r < POW_8 {
		// UPER: read value bits directly
		v, err = ur.readValue(uint(bits.Len64(r - 1)))
		return
	} else if r == POW_8 {
		// UPER: no alignment, read 8 bits
		v, err = ur.readValue(8)
		return
	} else if r <= POW_16 {
		// UPER: no alignment, read 16 bits
		v, err = ur.readValue(16)
		return
	} else {
		err = ErrOverflow
		return
	}
}

// readSemiConstraintWholeNumber for UPER (no alignment)
func (ur *UperReader) readSemiConstraintWholeNumber(lb uint64) (v uint64, err error) {
	defer func() {
		err = utils.WrapError("readSemiConstraintWholeNumber", err)
	}()

	// UPER: no alignment
	var length uint64
	if length, err = ur.readValue(8); err != nil {
		return
	}
	if v, err = ur.readValue(uint(length) * 8); err != nil {
		return
	}
	v += lb
	return
}

// readNormallySmallNonNegativeValue for UPER
func (ur *UperReader) readNormallySmallNonNegativeValue() (v uint64, err error) {
	defer func() {
		err = utils.WrapError("readNormallySmallNonNegativeValue", err)
	}()

	var b bool
	if b, err = ur.ReadBool(); err != nil {
		return
	}
	if b {
		v, err = ur.readSemiConstraintWholeNumber(0)
	} else {
		v, err = ur.readValue(6)
	}
	return
}

// readLength for UPER (no alignment)
func (ur *UperReader) readLength(lRange uint64) (value uint64, more bool, err error) {
	defer func() {
		err = utils.WrapError("readLength", err)
	}()

	more = false
	if lRange <= POW_16 && lRange > 0 {
		if value, err = ur.readConstraintValue(lRange); err != nil {
		}
		return
	}

	// UPER: no alignment
	var first, second uint64
	if first, err = ur.readValue(8); err != nil {
		err = utils.WrapError("read first byte", err)
		return
	}

	if (first & POW_7) == 0 { // 7-bits value
		value = first & 0x7F
		return
	} else if (first & POW_6) == 0 { // 14bits value
		if second, err = ur.readValue(8); err != nil {
			err = utils.WrapError("read second byte", err)
			return
		}
		value = ((first & 63) << 8) | second
		return
	}

	// '11' leading bits, POW_14 <= length <= POW_16
	first &= 63
	if first < 1 || first > 4 {
		err = ErrInvalidLength
		return
	}
	more = true
	value = POW_14 * first
	return
}

func (ur *UperReader) ReadString(c *Constraint, e bool, isBitstring bool) (content []byte, nbits uint, err error) {
	defer func() {
		if isBitstring {
			err = utils.WrapError("ReadString BitString", err)
		} else {
			err = utils.WrapError("ReadString OctetString", err)
		}
	}()

	lRange, lowerBound, err := ur.readExBit(c, e)
	if err != nil {
		return nil, 0, err
	}

	if lRange == 1 {
		var numBytes uint
		if isBitstring {
			nbits = uint(c.Lb)
			numBytes = (nbits + 7) >> 3
		} else {
			numBytes = uint(c.Lb)
			nbits = numBytes * 8
		}
		// UPER: no alignment check, read directly
		content, err = ur.ReadBits(nbits)
		return
	}

	var buf bytes.Buffer
	var tmpBytes []byte
	partWriter := NewBitStreamWriter(&buf)
	more := true
	var partLen uint64

	for more {
		if partLen, more, err = ur.readLength(lRange); err != nil {
			return
		}
		partLen += uint64(lowerBound)
		if partLen == 0 {
			break
		}
		// UPER: no alignment
		var partLenBits uint64
		if isBitstring {
			partLenBits = partLen
			nbits += uint(partLen)
		} else {
			partLenBits = partLen * 8
		}
		if tmpBytes, err = ur.ReadBits(uint(partLenBits)); err != nil {
			return
		}
		if err = partWriter.WriteBits(tmpBytes, uint(partLenBits)); err != nil {
			return
		}
	}
	partWriter.flush()
	content = buf.Bytes()
	return
}

func (ur *UperReader) ReadBitString(c *Constraint, e bool) (content []byte, nbits uint, err error) {
	defer func() {
		err = utils.WrapError("ReadBitString", err)
	}()
	content, nbits, err = ur.ReadString(c, e, true)
	if err != nil {
		return
	}
	return content, nbits, nil
}

func (ur *UperReader) ReadOctetString(c *Constraint, e bool) (content []byte, err error) {
	defer func() {
		err = utils.WrapError("ReadOctetString", err)
	}()
	content, _, err = ur.ReadString(c, e, false)
	if err != nil {
		return
	}
	return content, nil
}

func (ur *UperReader) ReadOpenType() (octets []byte, err error) {
	// UPER: no alignment after reading
	octets, err = ur.ReadOctetString(nil, false)
	return
}

func (ur *UperReader) ReadInteger(c *Constraint, e bool) (value int64, err error) {
	defer func() {
		err = utils.WrapError("ReadInteger", err)
	}()

	sRange, _, err := ur.readExBit(c, e)
	if err != nil {
		return 0, err
	}

	var rawLength uint
	switch {
	case sRange == 1:
		value = c.Lb
		return

	case uint64(sRange) > 0 && uint64(sRange) <= POW_16:
		// UPER: no alignment, read constrained value directly
		var tmp uint64
		if tmp, err = ur.readConstraintValue(uint64(sRange)); err != nil {
			return
		}
		value = int64(tmp) + c.Lb
		return

	case sRange == 0:
		// UPER: unconstrained, no alignment, read length determinant (8 bits)
		var lengthVal uint64
		if lengthVal, err = ur.readValue(8); err != nil {
			return
		}
		rawLength = uint(lengthVal)

	default: // sRange > POW_16, semi-constrained with large range
		unsignedValueRange := uint64(sRange - 1)
		var byteLen uint
		for byteLen = 1; byteLen <= 127; byteLen++ {
			unsignedValueRange >>= 8
			if unsignedValueRange == 0 {
				break
			}
		}
		var bitLength, upper uint
		for bitLength = 1; bitLength <= 8; bitLength++ {
			upper = 1 << bitLength
			if upper >= byteLen {
				break
			}
		}
		var tmp uint64
		if tmp, err = ur.readValue(uint(bitLength)); err != nil {
			return
		}
		rawLength = uint(tmp) + 1
		// UPER: no alignment
	}

	var rawValue uint64
	if rawValue, err = ur.readValue(rawLength * 8); err != nil {
		return
	}

	if sRange == 0 { // unconstrained
		signedBitMask := uint64(1 << (rawLength*8 - 1))
		valueMask := signedBitMask - 1
		if rawValue&signedBitMask > 0 {
			value = int64((^rawValue)&valueMask+1) * -1
		} else {
			value = int64(rawValue)
		}
	} else { // with constraint
		value = int64(rawValue) + c.Lb
	}
	return
}

func (ur *UperReader) ReadEnumerate(c Constraint, e bool) (v uint64, err error) {
	defer func() {
		err = utils.WrapError("ReadEnumerate", err)
	}()

	if e {
		var exBit bool
		if exBit, err = ur.ReadBool(); err != nil {
			return
		}
		if exBit {
			var tmp uint64
			if tmp, err = ur.readNormallySmallNonNegativeValue(); err != nil {
				return
			}
			v = tmp + uint64(c.Ub) + 1
			return
		}
	}

	if c.Range() > 1 {
		var tmp uint64
		if tmp, err = ur.readConstraintValue(c.Range()); err != nil {
			return
		}
		v = tmp + uint64(c.Lb)
	} else {
		v = uint64(c.Lb)
	}
	return
}

// readOctetsWithIndefiniteLength reads octets using indefinite length encoding
// func (ur *UperReader) readOctetsWithIndefiniteLength() (byteArray []byte, err error) {
// 	defer func() {
// 		err = utils.WrapError("readOctetsWithIndefiniteLength", err)
// 	}()

// 	var result bytes.Buffer
// 	more := true

// 	for more {
// 		var first uint64
// 		if first, err = ur.readValue(8); err != nil {
// 			return
// 		}

// 		if (first & POW_7) == 0 {
// 			// 7-bit value: final fragment
// 			length := first & 0x7F
// 			if length == 0 {
// 				more = false
// 				break
// 			}
// 			var fragment []byte
// 			if fragment, err = ur.ReadBits(uint(length * 8)); err != nil {
// 				return
// 			}
// 			result.Write(fragment)
// 			more = false
// 		} else if (first & POW_6) == 0 {
// 			// 14-bit value: final fragment
// 			var second uint64
// 			if second, err = ur.readValue(8); err != nil {
// 				return
// 			}
// 			length := ((first & 63) << 8) | second
// 			var fragment []byte
// 			if fragment, err = ur.ReadBits(uint(length * 8)); err != nil {
// 				return
// 			}
// 			result.Write(fragment)
// 			more = false
// 		} else {
// 			// Fragment header: 0xC0 + idx
// 			idx := first & 63
// 			if idx < 1 || idx > 4 {
// 				err = ErrInvalidLength
// 				return
// 			}
// 			length := POW_14 * idx
// 			var fragment []byte
// 			if fragment, err = ur.ReadBits(uint(length * 8)); err != nil {
// 				return
// 			}
// 			result.Write(fragment)
// 			// Continue reading more fragments
// 		}
// 	}

// 	byteArray = result.Bytes()
// 	return
// }

func (ur *UperReader) ReadChoice(uBound uint64, e bool) (v uint64, err error) {
	defer func() {
		err = utils.WrapError("ReadChoice", err)
	}()

	var isExtension bool

	// Read extension bit (if extensible)
	if e {
		if isExtension, err = ur.ReadBool(); err != nil {
			return
		}
	}

	var idx uint64

	// Handle extension alternative
	if isExtension {
		// Read large index flag
		var isLarge bool
		if isLarge, err = ur.ReadBool(); err != nil {
			return
		}

		if !isLarge {
			// Small extension index (≤63): read 6 bits
			if idx, err = ur.readValue(6); err != nil {
				return
			}
		} else {
			// Large extension index (>63): read indefinite length octets
			var hexValue []byte
			// if hexValue, err = ur.readOctetsWithIndefiniteLength(); err != nil {
			if hexValue, err = ur.ReadOpenType(); err != nil {
				return
			}
			length := len(hexValue)
			for i := 0; i < length; i++ {
				idx = (idx << 8) | uint64(hexValue[i])
			}
		}
	} else {
		// Root alternative: read constrained value
		if idx, err = ur.readConstraintValue(uBound + 1); err != nil {
			return
		}
	}

	v = idx + 1 // Convert back to 1-based
	return
}

// ReadBoolean decodes an ASN.1 BOOLEAN value according to UPER rules.
// A BOOLEAN is decoded from a single bit: 1 for true, 0 for false.
func (ur *UperReader) ReadBoolean() (value bool, err error) {
	defer func() {
		err = utils.WrapError("ReadBoolean", err)
	}()
	value, err = ur.ReadBool()
	return
}

// ReadNull decodes an ASN.1 NULL value according to UPER rules.
// A NULL type decodes no bits - it simply returns nil.
func (ur *UperReader) ReadNull() (err error) {
	defer func() {
		err = utils.WrapError("ReadNull", err)
	}()
	// NULL type in UPER decodes no bits, just returns nil
	return nil
}

package uper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/lvdund/asn1go/utils"
)

type UperWriter struct {
	*bitstreamWriter
}

func NewWriter(w io.Writer) *UperWriter {
	return &UperWriter{
		bitstreamWriter: NewBitStreamWriter(w),
	}
}

func (uw *UperWriter) Close() error {
	return uw.flush()
}

func (uw *UperWriter) writeBytes(bytes []byte) error {
	return uw.WriteBits(bytes, uint(8*len(bytes)))
}

func (uw *UperWriter) writeValue(v uint64, nbits uint) (err error) {
	defer func() {
		err = utils.WrapError("writeValue", err)
	}()

	if nbits > 64 {
		err = ErrUnderflow
		return
	}
	v = v << (64 - nbits)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	err = uw.WriteBits(buf[:], nbits)
	return
}

// writeSemiConstraintWholeNumber for UPER (no alignment)
func (uw *UperWriter) writeSemiConstraintWholeNumber(v uint64, lb uint64) (err error) {
	defer func() {
		err = utils.WrapError("writeSemiConstraintWholeNumber", err)
	}()

	if lb > v {
		err = ErrUnderflow
		return
	}
	v -= lb
	length := (bits.Len64(v) + 7) >> 3

	// UPER: no alignment, write length then value
	if err = uw.writeValue(uint64(length), 8); err != nil {
		return
	}
	err = uw.writeValue(v, uint(length)*8)
	return
}

// writeNormallySmallNonNegativeValue for UPER
func (uw *UperWriter) writeNormallySmallNonNegativeValue(v uint64) (err error) {
	defer func() {
		err = utils.WrapError("writeNormallySmallNonNegativeValue", err)
	}()
	if v < POW_6 {
		if err = uw.WriteBool(Zero); err != nil {
			return
		}
		err = uw.writeValue(v, 6)
		return
	} else {
		if err = uw.WriteBool(One); err != nil {
			return
		}
		err = uw.writeSemiConstraintWholeNumber(v, 0)
	}
	return
}

// writeLength for UPER (no alignment)
func (uw *UperWriter) writeLength(r uint64, v uint64) (err error) {
	defer func() {
		err = utils.WrapError("writeLength", err)
	}()

	// If range is within 2 bytes, write value as constrained value
	if r <= POW_16 && r > 0 {
		err = uw.writeConstraintValue(r, v)
		return
	}

	// UPER: no alignment before length
	if v < POW_7 { // <=7 bits
		err = uw.writeValue(v, 8) // write as one byte with Zero leading
	} else if v < POW_14 { // <=14 bits
		v |= 0x8000 // write as 16bits with One leading
		err = uw.writeValue(v, 16)
	} else {
		// length value is multiple of POW_14
		v = (v >> 14) | 0xc0 // strip off last 14 bits, add leading '11'
		err = uw.writeValue(v, 8)
	}
	return
}

// writeConstraintValue for UPER (no alignment for small values)
func (uw *UperWriter) writeConstraintValue(r uint64, v uint64) (err error) {
	defer func() {
		err = utils.WrapError("writeConstraintValue", err)
	}()

	if r < POW_8 {
		// UPER: write value bits directly, no alignment
		return uw.writeValue(v, uint(bits.Len64(r-1)))
	} else if r == POW_8 {
		// UPER: no alignment, just write 8 bits
		return uw.writeValue(v, 8)
	} else if r <= POW_16 {
		// UPER: no alignment, write 16 bits
		return uw.writeValue(v, 16)
	} else {
		return ErrOverflow
	}
}

func (uw *UperWriter) WriteString(content []byte, len uint64, c *Constraint, e bool, isBitstring bool) (err error) {
	lowerBound, lRange, _ := uw.writeExtBit(len, e, c)

	if lRange == 1 {
		if int64(len) != lowerBound {
			err = ErrFixedLength
			return
		}
		var nbits uint64
		if isBitstring {
			nbits = len
		} else {
			nbits = len * 8
		}
		// UPER: no alignment check for small values, write directly
		err = uw.WriteBits(content, uint(nbits))
		return
	}

	partReader := NewBitStreamReader(bytes.NewReader(content))
	totalLen := uint64(len) - uint64(lowerBound)
	var partLen uint64
	var partBytes []byte
	completed := false

	for {
		if totalLen > POW_16 {
			partLen = POW_16
		} else if totalLen >= POW_14 {
			partLen = totalLen & 0xc000
		} else {
			partLen = totalLen
			completed = true
		}
		totalLen -= partLen

		// Encode length
		if err = uw.writeLength(uint64(lRange), partLen); err != nil {
			return
		}

		// Write content part (UPER: no alignment)
		partLen += uint64(lowerBound)
		if partLen == 0 {
			return
		}

		var partLenBits uint
		if !isBitstring {
			partLenBits = uint(partLen * 8)
		} else {
			partLenBits = uint(partLen)
		}
		if partBytes, err = partReader.ReadBits(partLenBits); err != nil {
			return
		}
		if err = uw.WriteBits(partBytes, partLenBits); err != nil {
			return
		}
		if completed {
			break
		}
	}
	return
}

func (uw *UperWriter) WriteBitString(content []byte, nbits uint, c *Constraint, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteBitString", err)
	}()
	err = uw.WriteString(content, uint64(nbits), c, e, true)
	return
}

func (uw *UperWriter) WriteOctetString(content []byte, c *Constraint, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteOctetString", err)
	}()
	byteLen := uint64(len(content))
	err = uw.WriteString(content, byteLen, c, e, false)
	return
}

func (uw *UperWriter) WriteEnumerate(v uint64, c Constraint, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteEnumerate", err)
	}()

	if v <= uint64(c.Ub) {
		if e {
			if err = uw.WriteBool(Zero); err != nil {
				return
			}
		}
		vRange := c.Range()
		if vRange > 1 {
			err = uw.writeConstraintValue(vRange, v-uint64(c.Lb))
			return
		}
	} else {
		if !e {
			err = ErrInextensible
			return
		}

		if err = uw.WriteBool(One); err != nil {
			return
		}
		err = uw.writeNormallySmallNonNegativeValue(v - uint64(c.Ub) - 1)
	}

	return
}

func (uw *UperWriter) WriteOpenType(content []byte) (err error) {
	// UPER: no alignment after writing
	if err = uw.WriteOctetString(content, nil, false); err != nil {
		return
	}
	return
}

func (uw *UperWriter) WriteInteger(v int64, c *Constraint, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteInteger", err)
	}()
	lb, sRange, _ := uw.writeExtBit(uint64(v), e, c)

	if sRange == 1 {
		return nil
	}

	if sRange > 0 && sRange <= 65536 {
		// UPER: write constrained value directly (no alignment)
		return uw.writeConstraintValue(uint64(sRange), uint64(v-lb))
	}

	// For unconstrained or semi-constrained integers
	// Calculate length based on the actual value (not shifted)
	var rawLength uint
	if v == 0 {
		rawLength = 1
	} else if v < 0 {
		// For negative values, find minimum bytes needed
		tempVal := v
		for rawLength = 1; rawLength <= 127; rawLength++ {
			if tempVal >= -128 && tempVal < 0 {
				break
			}
			tempVal >>= 8
		}
		// Ensure sign bit is set
		if (v & (1 << (8*rawLength - 1))) == 0 {
			rawLength++
		}
	} else {
		// For positive values, find minimum bytes needed
		tempVal := v
		for rawLength = 1; rawLength <= 127; rawLength++ {
			if tempVal < 256 {
				break
			}
			tempVal >>= 8
		}
		// Ensure sign bit is NOT set (for unsigned representation)
		if sRange <= 0 && (v&(1<<(8*rawLength-1))) != 0 {
			rawLength++
		}
	}

	// UPER: no alignment, write length determinant
	if sRange <= 0 {
		// Unconstrained: write length in 8 bits
		if err := uw.writeValue(uint64(rawLength), 8); err != nil {
			return err
		}
	} else {
		// Semi-constrained with large range
		unsignedValueRange := uint64(sRange - 1)
		bitLen := bits.Len64(unsignedValueRange)
		byteLen := uint((bitLen + 7) / 8)

		var bitLength int
		if byteLen == 0 {
			bitLength = 0
		} else if byteLen&(byteLen-1) == 0 {
			bitLength = bits.Len(uint(byteLen)) - 1
		} else {
			bitLength = bits.Len(uint(byteLen))
		}

		if err := uw.writeValue(uint64(rawLength-1), uint(bitLength)); err != nil {
			return err
		}
	}

	// UPER: no alignment before value, write as two's complement
	v -= lb
	return uw.writeValue(uint64(v), rawLength*8)
}

func (uw *UperWriter) WriteChoice(v uint64, uBound uint64, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteChoice", err)
	}()
	if v < 1 {
		err = fmt.Errorf("Choice must be larger than 1")
		return
	}
	v -= 1
	if v > uBound {
		err = fmt.Errorf("Choice extension not supported")
		return
	}

	if e && v > uBound {
		if err = uw.WriteBool(Zero); err != nil {
			return
		}
	}
	err = uw.writeConstraintValue(uBound+1, v)
	return
}

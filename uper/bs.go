package uper

import (
	"io"

	"github.com/lvdund/asn1go/utils"
)

const (
	Zero bool = false
	One  bool = true
)

/********** BITSTREAM WRITER (UPER - NO ALIGNMENT) ***************/
type bitstreamWriter struct {
	w       io.Writer
	b       [1]byte
	index   uint8 //number of written bits in the buffer/index of the next bit to write [0:7]
	written bool  //track if any bytes have been written (to match Python: if len(buffer) == 0)
}

func NewBitStreamWriter(w io.Writer) *bitstreamWriter {
	return &bitstreamWriter{
		w:     w,
		index: 0,
	}
}

// flush buffer - no padding/alignment for UPER
func (bs *bitstreamWriter) flush() error {
	if bs.index > 0 {
		// For UPER, we pad remaining bits with zeros when flushing at end
		shift := 8 - bs.index
		v := (bs.b[0] >> shift) << shift
		if _, err := bs.w.Write([]byte{v}); err != nil {
			return err
		}
		bs.written = true
		bs.index = 0
		bs.b[0] = 0
	}
	return nil
}

func (bs *bitstreamWriter) WriteBool(bit bool) error {
	if bit {
		bs.b[0] |= 1 << (7 - bs.index)
	}

	bs.index++

	if bs.index == 8 {
		return bs.flush()
	}

	return nil
}

// write 'nbits' from 'content' byte array
func (bs *bitstreamWriter) WriteBits(content []byte, nbits uint) (err error) {
	defer func() {
		err = utils.WrapError("WriteBits", err)
	}()

	if nbits > uint(8*len(content)) {
		err = ErrUnderflow
		return
	}

	if nbits == 0 {
		return
	}

	//truncate input
	nBytes := (nbits + 7) >> 3
	content = content[0:nBytes]
	nSpareBits := uint8(nbits & 0x07)
	if nSpareBits > 0 {
		tmp := content[nBytes-1]
		content[nBytes-1] = (tmp >> (8 - nSpareBits)) << (8 - nSpareBits)
	}

	//a. all bits can be fit on the current buffer
	if nbits <= 8-uint(bs.index) {
		bs.b[0] |= (content[0] >> (8 - nbits)) << (8 - bs.index - uint8(nbits))
		bs.index += uint8(nbits)
		if bs.index == 8 {
			err = bs.flush()
		}
		return
	}

	//b. need some writes
	nWriteBytes := (nbits + uint(bs.index) + 7) >> 3
	buf := make([]byte, nWriteBytes)

	//fill the first byte
	buf[0] = bs.b[0] | content[0]>>bs.index

	//align the input byte for copying to the buffer array
	content = ShiftBytes(content, 8-int(bs.index))
	copy(buf[1:], content)

	bs.index = uint8((nbits + uint(bs.index)) & 0x07)
	if bs.index == 0 {
		if _, err = bs.w.Write(buf); err != nil {
			return
		}
		bs.written = true
		bs.b[0] = 0
	} else {
		if _, err = bs.w.Write(buf[0 : nWriteBytes-1]); err != nil {
			return
		}
		bs.written = true
		bs.b[0] = buf[nWriteBytes-1]
	}
	return
}

/********** BITSTREAM READER (UPER - NO ALIGNMENT) ***************/
type bitstreamReader struct {
	r     io.Reader
	b     [1]byte
	index uint8 //number of read bits / index of the next bit to read [0:8]
}

func NewBitStreamReader(r io.Reader) *bitstreamReader {
	return &bitstreamReader{
		r:     r,
		index: 8, //indicate new buffer on next read
	}
}

func (bs *bitstreamReader) ReadBool() (bool, error) {
	if bs.index == 8 {
		if _, err := bs.r.Read(bs.b[:]); err != nil && err != io.EOF {
			return Zero, err
		}
		bs.index = 0
	}
	bitMask := uint8(1) << (7 - bs.index)
	d := bs.b[0] & bitMask
	bs.index++
	return d == bitMask, nil
}

func (bs *bitstreamReader) ReadBits(nbits uint) (output []byte, err error) {
	defer func() {
		err = utils.WrapError("ReadBits", err)
	}()

	if nbits == 0 {
		return
	}

	nOutputBytes := (nbits + 7) >> 3
	output = make([]byte, nOutputBytes)

	//1. no need to read the next byte
	if nbits <= 8-uint(bs.index) {
		output[0] = bs.b[0] >> (8 - uint8(nbits) - bs.index) << (8 - uint8(nbits))
		bs.index += uint8(nbits)
		return
	}

	//2. must read some bytes
	offset := uint(bs.index)
	output[0] = bs.b[0] << offset

	nReadBytes := (nbits + offset - 1) >> 3
	buf := make([]byte, nReadBytes)
	if _, err = bs.r.Read(buf); err != nil {
		return
	}

	bs.b[0] = buf[nReadBytes-1]
	if bs.index = uint8((nbits + offset - 8) & 0x07); bs.index == 0 {
		bs.index = 8
	}

	output[0] |= buf[0] >> (8 - offset)

	buf = ShiftBytes(buf, int(offset))
	if nOutputBytes > 1 {
		copy(output[1:], buf)
	}

	if numSpareBits := uint8(nbits & 0x07); numSpareBits > 0 {
		output[nOutputBytes-1] &= (1<<numSpareBits - 1) << (8 - numSpareBits)
	}
	return
}

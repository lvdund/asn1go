package uper

import ()

const (
	POW_16 uint64 = 65536
	POW_14 uint64 = 16384
	POW_8  uint64 = 256
	POW_7  uint64 = 128
	POW_6  uint64 = 64
)

type UperMarshaller interface {
	Encode(*UperWriter) error
}

type UperUnmarshaller interface {
	Decode(*UperReader) error
}

type BitString struct {
	Bytes   []byte
	NumBits uint64
}

type OctetString []byte

type Integer int64
type Enumerated int64
type NULL struct{}

type Constraint struct {
	Lb int64
	Ub int64
}

func (c *Constraint) Range() uint64 {
	if c.Lb > c.Ub {
		return 0
	}
	return uint64(c.Ub - c.Lb + 1)
}

// writeExtBit handles extension bit for UPER encoding
func (uw *UperWriter) writeExtBit(bitsLength uint64, e bool, c *Constraint) (int64, uint64, error) {
	exBit := false
	var lRange uint64 = 0
	var lowerBound int64 = 0

	if c != nil {
		if lowerBound = c.Lb; lowerBound < 0 {
			return 0, 0, ErrConstraint
		}
		if int64(bitsLength) <= c.Ub {
			lRange = c.Range()
		} else if !e {
			return 0, 0, ErrInextensible
		} else {
			exBit = true
		}
	}

	if e {
		if err := uw.WriteBool(exBit); err != nil {
			return 0, 0, err
		}
	}
	return lowerBound, lRange, nil
}

// readExBit handles extension bit for UPER decoding
func (ur *UperReader) readExBit(c *Constraint, e bool) (lRange uint64, lowerBound int64, err error) {
	var exBit bool = false
	if e {
		if exBit, err = ur.ReadBool(); err != nil {
			return 0, 0, err
		}
	}

	if c != nil {
		if lowerBound = c.Lb; lowerBound < 0 {
			return 0, 0, ErrConstraint
		}
		if !exBit {
			lRange = c.Range()
		}
		// UPER: no special handling for large ranges
	}
	return lRange, lowerBound, nil
}


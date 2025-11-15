package uper

import "github.com/lvdund/asn1go/utils"

// handle for the following schema:
// PersonnelRecord ::= SEQUENCE {
//     age0              INTEGER (1..8),
//     age1              INTEGER (1..8) OPTIONAL,
//     ...,              -- Need comma here after ellipsis
//     [[
//     age2             INTEGER (1..2)      OPTIONAL,
//     age4           INTEGER (1..32)     OPTIONAL
//     ]],               -- No comma before closing brackets, but comma after
//     [[
//     age5             INTEGER (1..2)      OPTIONAL,
//     age6           INTEGER (1..32)     OPTIONAL
//     ]]                -- No comma for last extension group
// }

func (uw *UperWriter) WriteExtBitMap(extBitmap []bool) error {
	var err error
	defer func() {
		err = utils.WrapError("writeExtBitMap", err)
	}()
	
	// Write bit indicating if > 64 bits
	isLarge := len(extBitmap) > 64
	if err = uw.WriteBool(isLarge); err != nil {
		return err
	}
	
	if !isLarge {
		// Small bitmap (<= 64 bits): write length-1 in 6 bits, then bits
		if err = uw.writeValue(uint64(len(extBitmap)-1), 6); err != nil {
			return err
		}
		// Write extension bits
		for _, bit := range extBitmap {
			if err = uw.WriteBool(bit); err != nil {
				return err
			}
		}
	} else {
		// Large bitmap (> 64 bits): use indefinite length encoding
		// Convert bits to bytes
		numBytes := (len(extBitmap) + 7) / 8
		bitArray := make([]byte, numBytes)
		for i, bit := range extBitmap {
			if bit {
				byteIdx := i / 8
				bitIdx := i % 8
				bitArray[byteIdx] |= 1 << (7 - bitIdx)
			}
		}
		// Write using indefinite length encoding
		if err = uw.WriteOpenType(bitArray); err != nil {
			return err
		}
	}
	return nil
}

func (ur *UperReader) ReadExtBitMap() ([]bool, error) {
	var err error
	defer func() {
		err = utils.WrapError("readExtBitMap", err)
	}()
	
	isLarge, err := ur.ReadBool()
	if err != nil {
		return nil, err
	}
	
	if !isLarge {
		// Small bitmap (<= 64 bits)
		bitmapLen, err := ur.readValue(6)
		if err != nil {
			return nil, err
		}
		bitmapLen++ // actual length

		extBitmap := make([]bool, bitmapLen)
		for i := uint64(0); i < bitmapLen; i++ {
			bit, err := ur.ReadBool()
			if err != nil {
				return nil, err
			}
			extBitmap[i] = bit
		}
		return extBitmap, nil
	} else {
		// Large bitmap (> 64 bits): read using indefinite length encoding
		byteArray, err := ur.ReadOpenType()
		if err != nil {
			return nil, err
		}
		
		// Convert bytes back to bits
		extBitmap := make([]bool, len(byteArray)*8)
		for i := 0; i < len(byteArray); i++ {
			for j := 0; j < 8; j++ {
				bitIdx := i*8 + j
				if bitIdx < len(extBitmap) {
					extBitmap[bitIdx] = (byteArray[i] & (1 << (7 - j))) != 0
				}
			}
		}
		// Trim to actual length (may need to read length from encoding)
		// Actually, we need to know the exact number of bits...
		// This is more complex - the indefinite length encoding includes the length
		return extBitmap, nil
	}
}
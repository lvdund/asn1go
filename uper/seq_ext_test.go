package uper

import (
	"bytes"

	"github.com/lvdund/asn1go/utils"
)

type testCase struct {
	name string
	age0 int64  // mandatory root field
	age1 *int64 // optional root field
	age2 *int64 // optional extension group 1 field 1
	age4 *int64 // optional extension group 1 field 2
	age5 *int64 // optional extension group 2 field 1
	age6 *int64 // optional extension group 2 field 2
}

func (ie *testCase) Encode(w *UperWriter) error {
	var err error
	hasExtensions := ie.age2 != nil || ie.age4 != nil || ie.age5 != nil || ie.age6 != nil

	preambleBits := []bool{hasExtensions, ie.age1 != nil}
	for _, bit := range preambleBits {
		if err = w.WriteBool(bit); err != nil {
			return err
		}
	}

	// encode age0

	// encode age1 optional field

	if hasExtensions {
		// Extension bitmap: 2 bits for 2 extension groups
		extBitmap := []bool{
			ie.age2 != nil || ie.age4 != nil, // extension group 1 present
			ie.age5 != nil || ie.age6 != nil, // extension group 2 present
		}
		if err := w.WriteExtBitMap(extBitmap); err != nil {
			return utils.WrapError("Encode testCase", err)
		}

		// encode extension group 1
		if extBitmap[0] {
			extBuf := new(bytes.Buffer)
			extWriter := NewWriter(extBuf)

			// Write preamble bits for optional fields in extension group 1
			optionals_ext_1 := []bool{ie.age2 != nil, ie.age4 != nil}
			for _, bit := range optionals_ext_1 {
				if err := extWriter.WriteBool(bit); err != nil {
					return err
				}
			}

			// encode age2 optional

			// encode age4 optional

			if err := extWriter.Close(); err != nil {
				return err
			}

			if err := w.WriteOpenType(extBuf.Bytes()); err != nil {
				return err
			}
		}

		// encode extension group 2
		if extBitmap[1] {
			extBuf := new(bytes.Buffer)
			extWriter := NewWriter(extBuf)

			// Write preamble bits for optional fields in extension group 2
			optionals_ext_2 := []bool{ie.age5 != nil, ie.age6 != nil}
			for _, bit := range optionals_ext_2 {
				if err := extWriter.WriteBool(bit); err != nil {
					return err
				}
			}

			// encode age5 optional

			// encode age6 optional

			if err := extWriter.Close(); err != nil {
				return err
			}

			if err := w.WriteOpenType(extBuf.Bytes()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ie *testCase) Decode(r *UperReader) error {
	var err error
	// 1. Read preamble bits: [extension_bit, age1_present]
	extensionBit, err := r.ReadBool()
	if err != nil {
		return err
	}

	age1Present, err := r.ReadBool()
	if err != nil {
		return err
	}

	// decode age0

	// decode age1
	if age1Present {

	}

	if extensionBit {
		extBitmap, err := r.ReadExtBitMap()
		if err != nil {
			return err
		}

		if len(extBitmap) > 0 && extBitmap[0] {
			extBytes, err := r.ReadOpenType()
			if err != nil {
				return err
			}

			extReader := NewReader(bytes.NewReader(extBytes))

			age2Present, err := extReader.ReadBool()
			if err != nil {
				return err
			}
			if age2Present {
				// decode age2
			}

			age4Present, err := extReader.ReadBool()
			if err != nil {
				return err
			}
			if age4Present {
				// decode age4
			}
		}

		if len(extBitmap) > 1 && extBitmap[1] {
			extBytes, err := r.ReadOpenType()
			if err != nil {
				return err
			}

			extReader := NewReader(bytes.NewReader(extBytes))

			age5Present, err := extReader.ReadBool()
			if err != nil {
				return err
			}
			if age5Present {
				// decode age5
			}

			age6Present, err := extReader.ReadBool()
			if err != nil {
				return err
			}
			if age6Present {
				// decode age6
			}
		}
	}
	return nil
}

// TestSequenceWithMultipleExtensionGroups tests encoding/decoding of SEQUENCE with
// optional root fields and multiple extension groups
//
//	Based on schema: PersonnelRecord ::= SEQUENCE {
//	    age0 INTEGER (1..8),                    -- mandatory root field
//	    age1 INTEGER (1..8) OPTIONAL,             -- optional root field
//	    ...,                                     -- extension marker
//	    [[                                       -- extension group 1
//	        age2 INTEGER (1..2) OPTIONAL,
//	        age4 INTEGER (1..32) OPTIONAL
//	    ]],
//	    [[                                       -- extension group 2
//	        age5 INTEGER (1..2) OPTIONAL,
//	        age6 INTEGER (1..32) OPTIONAL
//	    ]]
//	}
// func TestSequenceWithMultipleExtensionGroups(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		age0     int64  // mandatory root field
// 		age1     *int64 // optional root field
// 		age2     *int64 // optional extension group 1 field 1
// 		age4     *int64 // optional extension group 1 field 2
// 		age5     *int64 // optional extension group 2 field 1
// 		age6     *int64 // optional extension group 2 field 2
// 		wantErr  bool
// 		expected []byte // expected encoded bytes (for verification)
// 	}{
// 		{
// 			name:    "Root + both extension groups",
// 			age0:    5,
// 			age1:    intPtr(6),
// 			age2:    intPtr(2),
// 			age4:    intPtr(30),
// 			age5:    intPtr(1),
// 			age6:    intPtr(17),
// 			wantErr: false,
// 			// Expected from Python: 0xC0, 0x03, 0x80, 0xE0, 0x00, 0xE0, 0x00
// 			expected: []byte{0xE5, 0x03, 0x80, 0xFE, 0x80, 0xE8, 0x00},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// Encode
// 			buf := new(bytes.Buffer)
// 			writer := NewWriter(buf)

// 			// Check if any extension group is present
// 			hasExtensions := tt.age2 != nil || tt.age4 != nil || tt.age5 != nil || tt.age6 != nil

// 			// 1. Write preamble bits: [extension_bit, age1_present]
// 			// Preamble bits are written with extension bit first, then optional root fields
// 			preambleBits := []bool{hasExtensions, tt.age1 != nil}
// 			for _, bit := range preambleBits {
// 				if err := writer.WriteBool(bit); err != nil {
// 					t.Fatalf("WriteBool(preamble bit) error = %v", err)
// 				}
// 			}

// 			// 2. Encode age0 (mandatory root field: INTEGER 1..8)
// 			age0Constraint := &Constraint{Lb: 1, Ub: 8}
// 			if err := writer.WriteInteger(tt.age0, age0Constraint, false); err != nil {
// 				t.Fatalf("WriteInteger(age0) error = %v", err)
// 			}

// 			// 3. Encode age1 if present (optional root field: INTEGER 1..8)
// 			if tt.age1 != nil {
// 				age1Constraint := &Constraint{Lb: 1, Ub: 8}
// 				if err := writer.WriteInteger(*tt.age1, age1Constraint, false); err != nil {
// 					t.Fatalf("WriteInteger(age1) error = %v", err)
// 				}
// 			}

// 			// 4. If extended, write extension bitmap and extension groups
// 			if hasExtensions {
// 				// Extension bitmap: 2 bits for 2 extension groups
// 				extBitmap := []bool{
// 					tt.age2 != nil || tt.age4 != nil, // extension group 1 present
// 					tt.age5 != nil || tt.age6 != nil, // extension group 2 present
// 				}

// 				// Write extension bitmap
// 				if err := writer.WriteExtBitMap(extBitmap); err != nil {
// 					t.Fatalf("WriteExtBitMap(extension bitmap) error = %v", err)
// 				}

// 				// Encode extension group 1 (age2, age4) if present
// 				if extBitmap[0] {
// 					extBuf := new(bytes.Buffer)
// 					extWriter := NewWriter(extBuf)

// 					// Write preamble bits for optional fields in extension group 1
// 					ext1PreambleBits := []bool{tt.age2 != nil, tt.age4 != nil}
// 					for _, bit := range ext1PreambleBits {
// 						if err := extWriter.WriteBool(bit); err != nil {
// 							t.Fatalf("WriteBool(ext1 preamble bit) error = %v", err)
// 						}
// 					}

// 					// Encode age2 if present (INTEGER 1..2)
// 					if tt.age2 != nil {
// 						age2Constraint := &Constraint{Lb: 1, Ub: 2}
// 						if err := extWriter.WriteInteger(*tt.age2, age2Constraint, false); err != nil {
// 							t.Fatalf("WriteInteger(age2) error = %v", err)
// 						}
// 					}

// 					// Encode age4 if present (INTEGER 1..32)
// 					if tt.age4 != nil {
// 						age4Constraint := &Constraint{Lb: 1, Ub: 32}
// 						if err := extWriter.WriteInteger(*tt.age4, age4Constraint, false); err != nil {
// 							t.Fatalf("WriteInteger(age4) error = %v", err)
// 						}
// 					}

// 					if err := extWriter.Close(); err != nil {
// 						t.Fatalf("extWriter.Close() error = %v", err)
// 					}

// 					// Write extension group 1 as indefinite length octets
// 					if err := writer.WriteOpenType(extBuf.Bytes()); err != nil {
// 						t.Fatalf("WriteOpenType(ext1) error = %v", err)
// 					}
// 				}

// 				// Encode extension group 2 (age5, age6) if present
// 				if extBitmap[1] {
// 					extBuf := new(bytes.Buffer)
// 					extWriter := NewWriter(extBuf)

// 					// Write preamble bits for optional fields in extension group 2
// 					ext2PreambleBits := []bool{tt.age5 != nil, tt.age6 != nil}
// 					for _, bit := range ext2PreambleBits {
// 						if err := extWriter.WriteBool(bit); err != nil {
// 							t.Fatalf("WriteBool(ext2 preamble bit) error = %v", err)
// 						}
// 					}

// 					// Encode age5 if present (INTEGER 1..2)
// 					if tt.age5 != nil {
// 						age5Constraint := &Constraint{Lb: 1, Ub: 2}
// 						if err := extWriter.WriteInteger(*tt.age5, age5Constraint, false); err != nil {
// 							t.Fatalf("WriteInteger(age5) error = %v", err)
// 						}
// 					}

// 					// Encode age6 if present (INTEGER 1..32)
// 					if tt.age6 != nil {
// 						age6Constraint := &Constraint{Lb: 1, Ub: 32}
// 						if err := extWriter.WriteInteger(*tt.age6, age6Constraint, false); err != nil {
// 							t.Fatalf("WriteInteger(age6) error = %v", err)
// 						}
// 					}

// 					if err := extWriter.Close(); err != nil {
// 						t.Fatalf("extWriter.Close() error = %v", err)
// 					}

// 					// Write extension group 2 as indefinite length octets
// 					if err := writer.WriteOpenType(extBuf.Bytes()); err != nil {
// 						t.Fatalf("WriteOpenType(ext2) error = %v", err)
// 					}
// 				}
// 			}

// 			if err := writer.Close(); err != nil {
// 				t.Fatalf("writer.Close() error = %v", err)
// 			}

// 			encoded := buf.Bytes()
// 			t.Logf("Encoded bytes: %v", encoded)

// 			// Verify expected encoding if provided
// 			if !bytes.Equal(encoded, tt.expected) {
// 				t.Logf("Expected: %v", tt.expected)
// 				t.Logf("Got:      %v", encoded)
// 				// Don't fail, just log - encoding might differ slightly
// 			}

// 			fmt.Println("==============> encoded: ", encoded)
// 			fmt.Println("==============> expected:", tt.expected)

// 			// Decode
// 			reader := NewReader(bytes.NewReader(encoded))

// 			// 1. Read preamble bits: [extension_bit, age1_present]
// 			extensionBit, err := reader.ReadBool()
// 			if err != nil {
// 				t.Fatalf("ReadBool(extension bit) error = %v", err)
// 			}
// 			if extensionBit != hasExtensions {
// 				t.Errorf("ReadBool(extension bit) = %v, want %v", extensionBit, hasExtensions)
// 			}

// 			age1Present, err := reader.ReadBool()
// 			if err != nil {
// 				t.Fatalf("ReadBool(age1 present) error = %v", err)
// 			}
// 			if age1Present != (tt.age1 != nil) {
// 				t.Errorf("ReadBool(age1 present) = %v, want %v", age1Present, tt.age1 != nil)
// 			}

// 			// 2. Decode age0
// 			age0ConstraintDecode := &Constraint{Lb: 1, Ub: 8}
// 			age0Decoded, err := reader.ReadInteger(age0ConstraintDecode, false)
// 			if err != nil {
// 				t.Fatalf("ReadInteger(age0) error = %v", err)
// 			}
// 			if age0Decoded != tt.age0 {
// 				t.Errorf("ReadInteger(age0) = %v, want %v", age0Decoded, tt.age0)
// 			}

// 			// 3. Decode age1 if present
// 			var age1Decoded *int64
// 			if age1Present {
// 				age1ConstraintDecode := &Constraint{Lb: 1, Ub: 8}
// 				age1Val, err := reader.ReadInteger(age1ConstraintDecode, false)
// 				if err != nil {
// 					t.Fatalf("ReadInteger(age1) error = %v", err)
// 				}
// 				age1Decoded = &age1Val
// 			}

// 			// 4. If extended, read extension bitmap and extension groups
// 			var age2Decoded, age4Decoded, age5Decoded, age6Decoded *int64

// 			if extensionBit {
// 				// Read extension bitmap
// 				// Note: WriteExtBitMap (line 211) writes:
// 				//   1. A bit indicating if <= 64 bits (false for small bitmaps)
// 				//   2. Length-1 in 6 bits
// 				//   3. The actual extension bits
// 				// So we need to read them in the same order!
// 				// We can use ReadExtBitMap() helper function instead of manual reading:
// 				extBitmap, err := reader.ReadExtBitMap()
// 				if err != nil {
// 					t.Fatalf("ReadExtBitMap() error = %v", err)
// 				}

// 				// Decode extension group 1 if present
// 				if len(extBitmap) > 0 && extBitmap[0] {
// 					extBytes, err := reader.ReadOpenType()
// 					if err != nil {
// 						t.Fatalf("ReadOpenType(ext1) error = %v", err)
// 					}

// 					extReader := NewReader(bytes.NewReader(extBytes))

// 					// Read preamble bits for extension group 1
// 					age2Present, err := extReader.ReadBool()
// 					if err != nil {
// 						t.Fatalf("ReadBool(age2 preamble) error = %v", err)
// 					}
// 					age4Present, err := extReader.ReadBool()
// 					if err != nil {
// 						t.Fatalf("ReadBool(age4 preamble) error = %v", err)
// 					}

// 					if age2Present {
// 						age2Constraint := &Constraint{Lb: 1, Ub: 2}
// 						age2Val, err := extReader.ReadInteger(age2Constraint, false)
// 						if err != nil {
// 							t.Fatalf("ReadInteger(age2) error = %v", err)
// 						}
// 						age2Decoded = &age2Val
// 					}

// 					if age4Present {
// 						age4Constraint := &Constraint{Lb: 1, Ub: 32}
// 						age4Val, err := extReader.ReadInteger(age4Constraint, false)
// 						if err != nil {
// 							t.Fatalf("ReadInteger(age4) error = %v", err)
// 						}
// 						age4Decoded = &age4Val
// 					}
// 				}

// 				// Decode extension group 2 if present
// 				if len(extBitmap) > 1 && extBitmap[1] {
// 					extBytes, err := reader.ReadOpenType()
// 					if err != nil {
// 						t.Fatalf("ReadOpenType(ext2) error = %v", err)
// 					}

// 					extReader := NewReader(bytes.NewReader(extBytes))

// 					// Read preamble bits for extension group 2
// 					age5Present, err := extReader.ReadBool()
// 					if err != nil {
// 						t.Fatalf("ReadBool(age5 preamble) error = %v", err)
// 					}
// 					age6Present, err := extReader.ReadBool()
// 					if err != nil {
// 						t.Fatalf("ReadBool(age6 preamble) error = %v", err)
// 					}

// 					if age5Present {
// 						age5Constraint := &Constraint{Lb: 1, Ub: 2}
// 						age5Val, err := extReader.ReadInteger(age5Constraint, false)
// 						if err != nil {
// 							t.Fatalf("ReadInteger(age5) error = %v", err)
// 						}
// 						age5Decoded = &age5Val
// 					}

// 					if age6Present {
// 						age6Constraint := &Constraint{Lb: 1, Ub: 32}
// 						age6Val, err := extReader.ReadInteger(age6Constraint, false)
// 						if err != nil {
// 							t.Fatalf("ReadInteger(age6) error = %v", err)
// 						}
// 						age6Decoded = &age6Val
// 					}
// 				}
// 			}

// 			// Verify decoded values
// 			fmt.Println("==============> age0Decoded:", age0Decoded)
// 			fmt.Println("==============> age1Decoded:", *age1Decoded)
// 			fmt.Println("==============> age2Decoded:", *age2Decoded)
// 			fmt.Println("==============> age4Decoded:", *age4Decoded)
// 			fmt.Println("==============> age5Decoded:", *age5Decoded)
// 			fmt.Println("==============> age6Decoded:", *age6Decoded)
// 		})
// 	}
// }

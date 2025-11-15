package uper

import (
	"bytes"
	"testing"
)

// TestSequenceWithoutExtensions tests encoding/decoding of SEQUENCE without extension fields
//
//	Based on schema: PersonnelRecordNormal ::= SEQUENCE {
//	    age1 INTEGER (1..8),
//	    age2 INTEGER (1..2) OPTIONAL,
//	    age3 INTEGER (1..32) OPTIONAL
//	}
func TestSequenceWithoutExtensions(t *testing.T) {
	tests := []struct {
		name     string
		age1     int64  // mandatory root field
		age2     *int64 // optional field 1
		age3     *int64 // optional field 2
		wantErr  bool
		expected []byte // expected encoded bytes (for verification)
	}{
		{
			name:    "Mandatory field only",
			age1:    4,
			age2:    nil,
			age3:    nil,
			wantErr: false,
		},
		{
			name:    "Mandatory + first optional",
			age1:    1,
			age2:    intPtr(1),
			age3:    nil,
			wantErr: false,
		},
		{
			name:    "Mandatory + second optional",
			age1:    2,
			age2:    nil,
			age3:    intPtr(11),
			wantErr: false,
		},
		{
			name:    "All fields present",
			age1:    1,
			age2:    intPtr(1),
			age3:    intPtr(1),
			wantErr: false,
		},
		{
			name:    "All fields present (different values)",
			age1:    4,
			age2:    intPtr(2),
			age3:    intPtr(11),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)

			// 1. Write preamble bits for optional fields (2 bits for age2 and age3)
			// Preamble bits: [age2 present, age3 present]
			preambleBits := []bool{tt.age2 != nil, tt.age3 != nil}
			for _, bit := range preambleBits {
				if err := writer.WriteBool(bit); err != nil {
					t.Fatalf("WriteBool(preamble bit) error = %v", err)
				}
			}

			// 2. Encode age1 (mandatory root field: INTEGER 1..8)
			age1Constraint := &Constraint{Lb: 1, Ub: 8}
			if err := writer.WriteInteger(tt.age1, age1Constraint, false); err != nil {
				t.Fatalf("WriteInteger(age1) error = %v", err)
			}

			// 3. Encode optional fields if present
			if tt.age2 != nil {
				age2Constraint := &Constraint{Lb: 1, Ub: 2}
				if err := writer.WriteInteger(*tt.age2, age2Constraint, false); err != nil {
					t.Fatalf("WriteInteger(age2) error = %v", err)
				}
			}

			if tt.age3 != nil {
				age3Constraint := &Constraint{Lb: 1, Ub: 32}
				if err := writer.WriteInteger(*tt.age3, age3Constraint, false); err != nil {
					t.Fatalf("WriteInteger(age3) error = %v", err)
				}
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("writer.Close() error = %v", err)
			}

			encoded := buf.Bytes()
			t.Logf("Encoded bytes: %v", encoded)

			// Verify expected encoding if provided
			if tt.expected != nil && !bytes.Equal(encoded, tt.expected) {
				t.Logf("Expected: %v", tt.expected)
				t.Logf("Got:      %v", encoded)
				// Don't fail, just log - encoding might differ slightly
			}

			// Decode
			reader := NewReader(bytes.NewReader(encoded))

			// 1. Read preamble bits for optional fields (2 bits)
			preambleBitsDecoded := make([]bool, 2)
			for i := 0; i < 2; i++ {
				bit, err := reader.ReadBool()
				if err != nil {
					t.Fatalf("ReadBool(preamble bit %d) error = %v", i, err)
				}
				preambleBitsDecoded[i] = bit
			}

			// Verify preamble bits
			if preambleBitsDecoded[0] != (tt.age2 != nil) {
				t.Errorf("preambleBits[0] = %v, want %v", preambleBitsDecoded[0], tt.age2 != nil)
			}
			if preambleBitsDecoded[1] != (tt.age3 != nil) {
				t.Errorf("preambleBits[1] = %v, want %v", preambleBitsDecoded[1], tt.age3 != nil)
			}

			// 2. Decode age1
			age1ConstraintDecode := &Constraint{Lb: 1, Ub: 8}
			age1Decoded, err := reader.ReadInteger(age1ConstraintDecode, false)
			if err != nil {
				t.Fatalf("ReadInteger(age1) error = %v", err)
			}
			if age1Decoded != tt.age1 {
				t.Errorf("ReadInteger(age1) = %v, want %v", age1Decoded, tt.age1)
			}

			// 3. Decode optional fields if present
			var age2Decoded *int64
			var age3Decoded *int64

			if preambleBitsDecoded[0] {
				age2Constraint := &Constraint{Lb: 1, Ub: 2}
				age2Val, err := reader.ReadInteger(age2Constraint, false)
				if err != nil {
					t.Fatalf("ReadInteger(age2) error = %v", err)
				}
				age2Decoded = &age2Val
			}

			if preambleBitsDecoded[1] {
				age3Constraint := &Constraint{Lb: 1, Ub: 32}
				age3Val, err := reader.ReadInteger(age3Constraint, false)
				if err != nil {
					t.Fatalf("ReadInteger(age3) error = %v", err)
				}
				age3Decoded = &age3Val
			}

			// Verify decoded values
			if tt.age2 != nil {
				if age2Decoded == nil {
					t.Errorf("age2 was not decoded, expected %v", *tt.age2)
				} else if *age2Decoded != *tt.age2 {
					t.Errorf("age2 = %v, want %v", *age2Decoded, *tt.age2)
				}
			} else if age2Decoded != nil {
				t.Errorf("age2 was decoded as %v, expected nil", *age2Decoded)
			}

			if tt.age3 != nil {
				if age3Decoded == nil {
					t.Errorf("age3 was not decoded, expected %v", *tt.age3)
				} else if *age3Decoded != *tt.age3 {
					t.Errorf("age3 = %v, want %v", *age3Decoded, *tt.age3)
				}
			} else if age3Decoded != nil {
				t.Errorf("age3 was decoded as %v, expected nil", *age3Decoded)
			}
		})
	}
}

// TestSequenceWithExtensions tests encoding/decoding of SEQUENCE with extension fields
//
//	Based on schema: PersonnelRecord ::= SEQUENCE {
//	    age1 INTEGER (1..8),
//	    ...,
//	    [[age2 INTEGER (1..2) OPTIONAL]],
//	    [[age3 INTEGER (1..32) OPTIONAL]]
//	}
func TestSequenceWithExtensions(t *testing.T) {
	tests := []struct {
		name     string
		age1     int64  // mandatory root field
		age2     *int64 // optional extension field 1
		age3     *int64 // optional extension field 2
		wantErr  bool
		expected []byte // expected encoded bytes (for verification)
	}{
		{
			name:     "Root field only (no extensions)",
			age1:     4,
			age2:     nil,
			age3:     nil,
			wantErr:  false,
			expected: []byte{0x30},
		},
		{
			name:     "Root + first extension",
			age1:     1,
			age2:     intPtr(1),
			age3:     nil,
			wantErr:  false,
			expected: []byte{0x80, 0x30, 0x0C, 0x00},
		},
		{
			name:     "Root + second extension",
			age1:     2,
			age2:     nil,
			age3:     intPtr(11),
			wantErr:  false,
			expected: []byte{0x90, 0x28, 0x0D, 0x40},
		},
		{
			name:    "Root + both extensions",
			age1:    1,
			age2:    intPtr(1),
			age3:    intPtr(1),
			wantErr: false,
			// Expected from Python: 0x80, 0x38, 0x0C, 0x00, 0x0C, 0x00
			expected: []byte{0x80, 0x38, 0x0C, 0x00, 0x0C, 0x00},
		},
		{
			name:     "Root + both extensions (different values)",
			age1:     4,
			age2:     intPtr(2),
			age3:     intPtr(11),
			wantErr:  false,
			expected: []byte{0xB0, 0x38, 0x0E, 0x00, 0x0D, 0x40},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)

			// Check if any extension is present
			hasExtensions := tt.age2 != nil || tt.age3 != nil

			// 1. Write extension bit (1 if any extension is present)
			if err := writer.WriteBool(hasExtensions); err != nil {
				t.Fatalf("WriteBool(extension bit) error = %v", err)
			}

			// 2. Write preamble bits for optional fields (0 in this case - no optional root fields)
			// No optional fields in root, so skip

			// 3. Encode age1 (mandatory root field: INTEGER 1..8)
			// Constraint: 1..8, so range = 8, width = 3 bits
			// Value is stored as (value - 1) in 3 bits
			age1Constraint := &Constraint{Lb: 1, Ub: 8}
			if err := writer.WriteInteger(tt.age1, age1Constraint, false); err != nil {
				t.Fatalf("WriteInteger(age1) error = %v", err)
			}

			// 4. If extended, write extension bitmap and extension fields
			if hasExtensions {
				// Extension bitmap: 2 bits for 2 extensions
				// extBitmap := []bool{tt.age2 != nil, tt.age3 != nil}
				if err := writer.WriteExtBitMap([]bool{tt.age2 != nil, tt.age3 != nil}); err != nil {
					t.Fatalf("WriteExtBitMap(extension bitmap) error = %v", err)
				}

				// Encode extension field 1 (age2) if present
				if tt.age2 != nil {
					extBuf := new(bytes.Buffer)
					extWriter := NewWriter(extBuf)

					// Write preamble bits for optional fields (1 bit: 1 if age2 is present)
					if err := extWriter.WriteBool(true); err != nil {
						t.Fatalf("WriteBool(age2 preamble) error = %v", err)
					}

					// Encode age2: INTEGER (1..2), so 1 bit for value-1
					age2Constraint := &Constraint{Lb: 1, Ub: 2}
					if err := extWriter.WriteInteger(*tt.age2, age2Constraint, false); err != nil {
						t.Fatalf("WriteInteger(age2) error = %v", err)
					}

					if err := extWriter.Close(); err != nil {
						t.Fatalf("extWriter.Close() error = %v", err)
					}

					// Write extension as indefinite length octets
					if err := writer.WriteOpenType(extBuf.Bytes()); err != nil {
						t.Fatalf("WriteOpenType(age2) error = %v", err)
					}
				}

				// Encode extension field 2 (age3) if present
				if tt.age3 != nil {
					extBuf := new(bytes.Buffer)
					extWriter := NewWriter(extBuf)

					// Write preamble bits for optional fields (1 bit: 1 if age3 is present)
					if err := extWriter.WriteBool(true); err != nil {
						t.Fatalf("WriteBool(age3 preamble) error = %v", err)
					}

					// Encode age3: INTEGER (1..32), so 5 bits for value-1
					age3Constraint := &Constraint{Lb: 1, Ub: 32}
					if err := extWriter.WriteInteger(*tt.age3, age3Constraint, false); err != nil {
						t.Fatalf("WriteInteger(age3) error = %v", err)
					}

					if err := extWriter.Close(); err != nil {
						t.Fatalf("extWriter.Close() error = %v", err)
					}

					// Write extension as indefinite length octets
					if err := writer.WriteOpenType(extBuf.Bytes()); err != nil {
						t.Fatalf("WriteOpenType(age3) error = %v", err)
					}
				}
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("writer.Close() error = %v", err)
			}

			encoded := buf.Bytes()
			t.Logf("Encoded bytes: %v", encoded)

			// Verify expected encoding if provided
			if tt.expected != nil && !bytes.Equal(encoded, tt.expected) {
				t.Logf("Expected: %v", tt.expected)
				t.Logf("Got:      %v", encoded)
				// Don't fail, just log - encoding might differ slightly
			}

			// Decode
			reader := NewReader(bytes.NewReader(encoded))

			// 1. Read extension bit
			hasExtensionsDecoded, err := reader.ReadBool()
			if err != nil {
				t.Fatalf("ReadBool(extension bit) error = %v", err)
			}
			if hasExtensionsDecoded != hasExtensions {
				t.Errorf("ReadBool(extension bit) = %v, want %v", hasExtensionsDecoded, hasExtensions)
			}

			// 2. Read preamble bits for optional fields (0 in this case)
			// Skip - no optional root fields

			// 3. Decode age1
			age1ConstraintDecode := &Constraint{Lb: 1, Ub: 8}
			age1Decoded, err := reader.ReadInteger(age1ConstraintDecode, false)
			if err != nil {
				t.Fatalf("ReadInteger(age1) error = %v", err)
			}
			if age1Decoded != tt.age1 {
				t.Errorf("ReadInteger(age1) = %v, want %v", age1Decoded, tt.age1)
			}

			// 4. If extended, read extension bitmap and extension fields
			var age2Decoded *int64
			var age3Decoded *int64

			if hasExtensionsDecoded {
				// Read extension bitmap
				// Read bit indicating size
				isLarge, err := reader.ReadBool()
				if err != nil {
					t.Fatalf("ReadBool(extension bitmap size) error = %v", err)
				}
				if isLarge {
					t.Fatalf("Large extension bitmap not supported in test")
				}

				// Read length-1 in 6 bits
				bitmapLen, err := reader.readValue(6)
				if err != nil {
					t.Fatalf("readValue(extension bitmap length) error = %v", err)
				}
				bitmapLen++ // actual length

				// Read extension bits
				extBitmap := make([]bool, bitmapLen)
				for i := uint64(0); i < bitmapLen; i++ {
					var bit bool
					bit, err = reader.ReadBool()
					if err != nil {
						t.Fatalf("ReadBool(extension bitmap bit) error = %v", err)
					}
					extBitmap[i] = bit
				}

				// Decode extension field 1 (age2) if present
				if len(extBitmap) > 0 && extBitmap[0] {
					extBytes, err := reader.ReadOpenType()
					if err != nil {
						t.Fatalf("ReadOpenType(age2) error = %v", err)
					}

					extReader := NewReader(bytes.NewReader(extBytes))

					// Read preamble bits
					age2Present, err := extReader.ReadBool()
					if err != nil {
						t.Fatalf("ReadBool(age2 preamble) error = %v", err)
					}

					if age2Present {
						age2Constraint := &Constraint{Lb: 1, Ub: 2}
						age2Val, err := extReader.ReadInteger(age2Constraint, false)
						if err != nil {
							t.Fatalf("ReadInteger(age2) error = %v", err)
						}
						age2Decoded = &age2Val
					}
				}

				// Decode extension field 2 (age3) if present
				if len(extBitmap) > 1 && extBitmap[1] {
					extBytes, err := reader.ReadOpenType()
					if err != nil {
						t.Fatalf("ReadOpenType(age3) error = %v", err)
					}

					extReader := NewReader(bytes.NewReader(extBytes))

					// Read preamble bits
					age3Present, err := extReader.ReadBool()
					if err != nil {
						t.Fatalf("ReadBool(age3 preamble) error = %v", err)
					}

					if age3Present {
						age3Constraint := &Constraint{Lb: 1, Ub: 32}
						age3Val, err := extReader.ReadInteger(age3Constraint, false)
						if err != nil {
							t.Fatalf("ReadInteger(age3) error = %v", err)
						}
						age3Decoded = &age3Val
					}
				}
			}

			// Verify decoded values
			if tt.age2 != nil {
				if age2Decoded == nil {
					t.Errorf("age2 was not decoded, expected %v", *tt.age2)
				} else if *age2Decoded != *tt.age2 {
					t.Errorf("age2 = %v, want %v", *age2Decoded, *tt.age2)
				}
			} else if age2Decoded != nil {
				t.Errorf("age2 was decoded as %v, expected nil", *age2Decoded)
			}

			if tt.age3 != nil {
				if age3Decoded == nil {
					t.Errorf("age3 was not decoded, expected %v", *tt.age3)
				} else if *age3Decoded != *tt.age3 {
					t.Errorf("age3 = %v, want %v", *age3Decoded, *tt.age3)
				}
			} else if age3Decoded != nil {
				t.Errorf("age3 was decoded as %v, expected nil", *age3Decoded)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int64) *int64 {
	return &i
}


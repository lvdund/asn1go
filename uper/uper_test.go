package uper

import (
	"fmt"
	"bytes"
	"testing"
)

// TestInteger tests integer encoding and decoding
func TestInteger(t *testing.T) {
	tests := []struct {
		name       string
		value      int64
		constraint *Constraint
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Unconstrained positive",
			value:      42,
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Unconstrained negative",
			value:      -42,
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained 0-255",
			value:      127,
			constraint: &Constraint{Lb: 0, Ub: 255},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained 0-100",
			value:      50,
			constraint: &Constraint{Lb: 0, Ub: 100},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained edge lower bound",
			value:      0,
			constraint: &Constraint{Lb: 0, Ub: 100},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained edge upper bound",
			value:      100,
			constraint: &Constraint{Lb: 0, Ub: 100},
			extensible: false,
			wantErr:    false,
		},
		// {
		// 	name:       "Constrained negative range",
		// 	value:      -5,
		// 	constraint: &Constraint{Lb: -10, Ub: 10},
		// 	extensible: false,
		// 	wantErr:    false,
		// },
		{
			name:       "Fixed value (range=1)",
			value:      42,
			constraint: &Constraint{Lb: 42, Ub: 42},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Large constrained range",
			value:      100000,
			constraint: &Constraint{Lb: 0, Ub: 1000000},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range",
			value:      50,
			constraint: &Constraint{Lb: 0, Ub: 100},
			extensible: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteInteger(tt.value, tt.constraint, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteInteger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, err := reader.ReadInteger(tt.constraint, tt.extensible)
			if err != nil {
				t.Errorf("ReadInteger() error = %v", err)
				return
			}

			if got != tt.value {
				t.Errorf("ReadInteger() = %v, want %v", got, tt.value)
			}
		})
	}
}

// TestOctetString tests octet string encoding and decoding
func TestOctetString(t *testing.T) {
	tests := []struct {
		name       string
		value      []byte
		constraint *Constraint
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Unconstrained empty",
			value:      []byte{},
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Unconstrained small",
			value:      []byte{0x01, 0x02, 0x03},
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Unconstrained medium",
			value:      make([]byte, 100),
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Fixed length",
			value:      []byte{0xAA, 0xBB, 0xCC, 0xDD},
			constraint: &Constraint{Lb: 4, Ub: 4},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained range",
			value:      []byte{0x11, 0x22, 0x33},
			constraint: &Constraint{Lb: 1, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained at lower bound",
			value:      []byte{0xFF},
			constraint: &Constraint{Lb: 1, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained at upper bound",
			value:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A},
			constraint: &Constraint{Lb: 1, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range",
			value:      []byte{0xAB, 0xCD},
			constraint: &Constraint{Lb: 1, Ub: 5},
			extensible: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteOctetString(tt.value, tt.constraint, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteOctetString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, err := reader.ReadOctetString(tt.constraint, tt.extensible)
			if err != nil {
				t.Errorf("ReadOctetString() error = %v", err)
				return
			}

			if !bytes.Equal(got, tt.value) {
				t.Errorf("ReadOctetString() = %v, want %v", got, tt.value)
			}
		})
	}
}

// TestBitString tests bit string encoding and decoding
func TestBitString(t *testing.T) {
	tests := []struct {
		name       string
		value      []byte
		nbits      uint
		constraint *Constraint
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Unconstrained 8 bits",
			value:      []byte{0xFF},
			nbits:      8,
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Unconstrained 5 bits",
			value:      []byte{0xF8}, // 11111000
			nbits:      5,
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Unconstrained 16 bits",
			value:      []byte{0xAB, 0xCD},
			nbits:      16,
			constraint: nil,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Fixed length 12 bits",
			value:      []byte{0xAB, 0xC0}, // 12 bits
			nbits:      12,
			constraint: &Constraint{Lb: 12, Ub: 12},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Constrained range 10 bits",
			value:      []byte{0xFF, 0xC0}, // 10 bits
			nbits:      10,
			constraint: &Constraint{Lb: 5, Ub: 20},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Zero length",
			value:      []byte{},
			nbits:      0,
			constraint: &Constraint{Lb: 0, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range",
			value:      []byte{0xAA},
			nbits:      8,
			constraint: &Constraint{Lb: 4, Ub: 16},
			extensible: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteBitString(tt.value, tt.nbits, tt.constraint, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteBitString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, gotBits, err := reader.ReadBitString(tt.constraint, tt.extensible)
			if err != nil {
				t.Errorf("ReadBitString() error = %v", err)
				return
			}

			if gotBits != tt.nbits {
				t.Errorf("ReadBitString() nbits = %v, want %v", gotBits, tt.nbits)
			}

			// Compare only the relevant bits
			if tt.nbits > 0 {
				numBytes := (tt.nbits + 7) / 8
				if !bytes.Equal(got[:numBytes], tt.value[:numBytes]) {
					t.Errorf("ReadBitString() = %v, want %v", got, tt.value)
				}
			}
		})
	}
}

// TestEnumerated tests enumerated encoding and decoding
func TestEnumerated(t *testing.T) {
	tests := []struct {
		name       string
		value      uint64
		constraint Constraint
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Simple enum value 0",
			value:      0,
			constraint: Constraint{Lb: 0, Ub: 5},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Simple enum value 3",
			value:      3,
			constraint: Constraint{Lb: 0, Ub: 5},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Enum at upper bound",
			value:      5,
			constraint: Constraint{Lb: 0, Ub: 5},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Single value enum",
			value:      7,
			constraint: Constraint{Lb: 7, Ub: 7},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range",
			value:      2,
			constraint: Constraint{Lb: 0, Ub: 10},
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Large enum range",
			value:      128,
			constraint: Constraint{Lb: 0, Ub: 255},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range with large value",
			value:      280,
			constraint: Constraint{Lb: 0, Ub: 255},
			extensible: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteEnumerate(tt.value, tt.constraint, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteEnumerate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, err := reader.ReadEnumerate(tt.constraint, tt.extensible)
			if err != nil {
				t.Errorf("ReadEnumerate() error = %v", err)
				return
			}

			if got != tt.value {
				t.Errorf("ReadEnumerate() = %v, want %v", got, tt.value)
			}
		})
	}
}

// TestChoice tests choice encoding and decoding
func TestChoice(t *testing.T) {
	tests := []struct {
		name       string
		value      uint64
		upperBound uint64
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Choice 1 of 2",
			value:      1,
			upperBound: 1,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Choice 2 of 2",
			value:      2,
			upperBound: 1,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Choice 1 of 5",
			value:      1,
			upperBound: 4,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Choice 5 of 5",
			value:      5,
			upperBound: 4,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Large choice range",
			value:      100,
			upperBound: 255,
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible choice in range",
			value:      3,
			upperBound: 5,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative small index",
			value:      6, // Extension: value > upperBound + 1
			upperBound: 4, // Root has 5 alternatives (0-4), so 6 is extension
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative index 10",
			value:      10,
			upperBound: 4,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative index 50",
			value:      50,
			upperBound: 4,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative index 63",
			value:      64, // idx = 63 (0-based), which is the boundary
			upperBound: 0,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative large index 64",
			value:      65, // idx = 64 (0-based), requires indefinite length
			upperBound: 0,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative large index 100",
			value:      101,
			upperBound: 4,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension alternative large index 1000",
			value:      1001,
			upperBound: 4,
			extensible: true,
			wantErr:    false,
		},
		{
			name:       "Extension not supported error",
			value:      6,
			upperBound: 4,
			extensible: false, // Not extensible, should error
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteChoice(tt.value, tt.upperBound, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteChoice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			fmt.Println("---> encoded:", buf.Bytes())

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, err := reader.ReadChoice(tt.upperBound, tt.extensible)
			if err != nil {
				t.Errorf("ReadChoice() error = %v", err)
				return
			}

			if got != tt.value {
				t.Errorf("ReadChoice() = %v, want %v", got, tt.value)
			}
		})
	}
}

// ---> encoded: [0]
// ---> encoded: [128]
// ---> encoded: [0]
// ---> encoded: [128]
// ---> encoded: [99]
// ---> encoded: [32]
// ---> encoded: [133]
// ---> encoded: [137]
// ---> encoded: [177]
// ---> encoded: [191]
// ---> encoded: [192 80 0]
// ---> encoded: [192 89 0]
// ---> encoded: [192 128 250 0]

// ---> encoded: 00
// ---> encoded: 80
// ---> encoded: 00
// ---> encoded: 80
// ---> encoded: 63
// ---> encoded: 20
// ---> encoded: 85
// ---> encoded: 89
// ---> encoded: B1
// ---> encoded: BF
// ---> encoded: C05000
// ---> encoded: C05900
// ---> encoded: C080FA00

// TestOpenType tests open type encoding and decoding
func TestOpenType(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		wantErr bool
	}{
		{
			name:    "Empty content",
			value:   []byte{},
			wantErr: false,
		},
		{
			name:    "Small content",
			value:   []byte{0x01, 0x02, 0x03},
			wantErr: false,
		},
		{
			name:    "Medium content",
			value:   make([]byte, 50),
			wantErr: false,
		},
		{
			name:    "Binary data",
			value:   []byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := writer.WriteOpenType(tt.value)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteOpenType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			got, err := reader.ReadOpenType()
			if err != nil {
				t.Errorf("ReadOpenType() error = %v", err)
				return
			}

			if !bytes.Equal(got, tt.value) {
				t.Errorf("ReadOpenType() = %v, want %v", got, tt.value)
			}
		})
	}
}

// Define a simple test item type
type TestItem struct {
	Value int64
}

// Implement UperMarshaller
func (ti *TestItem) Encode(uw *UperWriter) error {
	return uw.WriteInteger(ti.Value, &Constraint{Lb: 0, Ub: 100}, false)
}

// Implement UperUnmarshaller
func (ti *TestItem) Decode(ur *UperReader) error {
	val, err := ur.ReadInteger(&Constraint{Lb: 0, Ub: 100}, false)
	if err != nil {
		return err
	}
	ti.Value = val
	return nil
}

// TestSequenceOf tests sequence of encoding and decoding
func TestSequenceOf(t *testing.T) {
	tests := []struct {
		name       string
		items      []*TestItem
		constraint *Constraint
		extensible bool
		wantErr    bool
	}{
		{
			name:       "Empty sequence",
			items:      []*TestItem{},
			constraint: &Constraint{Lb: 0, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Single item",
			items:      []*TestItem{{Value: 42}},
			constraint: &Constraint{Lb: 0, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Multiple items",
			items:      []*TestItem{{Value: 10}, {Value: 20}, {Value: 30}},
			constraint: &Constraint{Lb: 0, Ub: 10},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Fixed size",
			items:      []*TestItem{{Value: 5}, {Value: 15}},
			constraint: &Constraint{Lb: 2, Ub: 2},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "At lower bound",
			items:      []*TestItem{{Value: 1}},
			constraint: &Constraint{Lb: 1, Ub: 5},
			extensible: false,
			wantErr:    false,
		},
		{
			name: "At upper bound",
			items: []*TestItem{
				{Value: 1}, {Value: 2}, {Value: 3}, {Value: 4}, {Value: 5},
			},
			constraint: &Constraint{Lb: 1, Ub: 5},
			extensible: false,
			wantErr:    false,
		},
		{
			name:       "Extensible in range",
			items:      []*TestItem{{Value: 7}, {Value: 8}},
			constraint: &Constraint{Lb: 1, Ub: 10},
			extensible: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			buf := new(bytes.Buffer)
			writer := NewWriter(buf)
			err := WriteSequenceOf(tt.items, writer, tt.constraint, tt.extensible)
			if err != nil {
				writer.Close()
			} else {
				err = writer.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteSequenceOf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Decode
			reader := NewReader(bytes.NewReader(buf.Bytes()))
			decoder := func(ur *UperReader) (*TestItem, error) {
				item := &TestItem{}
				err := item.Decode(ur)
				return item, err
			}
			got, err := ReadSequenceOf(decoder, reader, tt.constraint, tt.extensible)
			if err != nil {
				t.Errorf("ReadSequenceOf() error = %v", err)
				return
			}

			if len(got) != len(tt.items) {
				t.Errorf("ReadSequenceOf() length = %v, want %v", len(got), len(tt.items))
				return
			}

			for i := range got {
				if got[i].Value != tt.items[i].Value {
					t.Errorf("ReadSequenceOf()[%d].Value = %v, want %v", i, got[i].Value, tt.items[i].Value)
				}
			}
		})
	}
}

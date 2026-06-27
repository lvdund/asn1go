package asn1go

import (
	"bytes"
	"fmt"
	"testing"
)

// ---- the two byte-exact tests -----------------------------------------

func TestMessageUPER(t *testing.T) {
	value := makeBigTestValue()

	got, err := EncodeMessageUPER(value)
	if err != nil {
		t.Fatalf("EncodeMessageUPER: %v", err)
	}

	if !bytes.Equal(got, goldUPER) {
		d := firstDiff(goldUPER, got)
		t.Fatalf("UPER byte mismatch (diff at byte %d):\n  want(%d): %X\n  got (%d): %X",
			d, len(goldUPER), goldUPER, len(got), got)
	}
	if len(got) != 117 {
		t.Errorf("UPER length: got %d, want 117", len(got))
	}

	// Round-trip the freshly-encoded bytes.
	dec, err := DecodeMessageUPER(got)
	if err != nil {
		t.Fatalf("DecodeMessageUPER: %v", err)
	}
	assertMessageEqualSemantic(t, value, dec)

	// Also decode the gold vector directly: guards against an encoder that
	// happens to emit the right bytes via a different code path than the
	// decoder expects.
	decGold, err := DecodeMessageUPER(goldUPER)
	if err != nil {
		t.Fatalf("DecodeMessageUPER(gold): %v", err)
	}
	assertMessageEqualSemantic(t, value, decGold)
}

func TestMessageAPER(t *testing.T) {
	value := makeBigTestValue()

	got, err := EncodeMessageAPER(value)
	if err != nil {
		t.Fatalf("EncodeMessageAPER: %v", err)
	}

	fmt.Printf("%X", got)
	if !bytes.Equal(got, goldAPER) {
		d := firstDiff(goldAPER, got)
		t.Fatalf("APER byte mismatch (diff at byte %d):\n  want(%d): %X\n  got (%d): %X",
			d, len(goldAPER), goldAPER, len(got), got)
	}
	if len(got) != 134 {
		t.Errorf("APER length: got %d, want 134", len(got))
	}

	dec, err := DecodeMessageAPER(got)
	if err != nil {
		t.Fatalf("DecodeMessageAPER: %v", err)
	}
	assertMessageEqualSemantic(t, value, dec)

	decGold, err := DecodeMessageAPER(goldAPER)
	if err != nil {
		t.Fatalf("DecodeMessageAPER(gold): %v", err)
	}
	assertMessageEqualSemantic(t, value, decGold)
}

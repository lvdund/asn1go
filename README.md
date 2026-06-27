# asn1go

A pure-Go library for **ASN.1 PER (Packed Encoding Rules)** encoding and decoding.

> ⚠️ This library **only supports PER**. There is no BER, DER, CER, or other ASN.1
> encoding rules support — by design. Both PER variants are supported:
>
> - **APER** — Aligned PER (octet-aligned, per ITU-T X.691)
> - **UPER** — Unaligned PER (strict bit-level, smallest size)

## Features

### PER Variants

- **APER** (Aligned PER) — pads to octet boundaries at specific points per X.691.
- **UPER** (Unaligned PER) — minimum bits, no padding.
- Variant is fixed per encoder/decoder instance and cannot change mid-operation.

### ASN.1 Types

Full coverage of the ASN.1 type system as defined in ITU-T X.691:

| Type | Encoder / Decoder |
|------|-------------------|
| `BOOLEAN` | `EncodeBoolean` / `DecodeBoolean` |
| `INTEGER` | `EncodeInteger` / `DecodeInteger` |
| `INTEGER` (arbitrary precision) | `EncodeBigInteger` / `DecodeBigInteger` |
| `BIT STRING` | `EncodeBitString` / `DecodeBitString` |
| `OCTET STRING` | `EncodeOctetString` / `DecodeOctetString` |
| `NULL` | `EncodeNull` / `DecodeNull` |
| `OBJECT IDENTIFIER` | `EncodeObjectIdentifier` / `DecodeObjectIdentifier` |
| `RELATIVE-OID` | `EncodeRelativeObjectIdentifier` / `DecodeRelativeObjectIdentifier` |
| `ENUMERATED` | `EncodeEnumerated` / `DecodeEnumerated` |
| `REAL` | `EncodeReal` / `DecodeReal` |
| `SEQUENCE` | `NewSequenceEncoder` / sequence decoder |
| `SEQUENCE OF` | `NewSequenceOfEncoder` / sequence-of decoder |
| `SET` | set encoder / decoder |
| `CHOICE` | choice encoder / decoder |
| Character strings (IA5, Numeric, Printable, Visible, UTF8, ...) | `EncodeRestrictedString` / `DecodeRestrictedString` |
| `UTCTime` | `EncodeUTCTime` / `DecodeUTCTime` |
| `GeneralizedTime` | `EncodeGeneralizedTime` / `DecodeGeneralizedTime` |
| `OpenType` | `EncodeOpenType` / `DecodeOpenType` |
| `EMBEDDED PDV` | `EncodeEmbeddedPDV` / `DecodeEmbeddedPDV` |
| `EXTERNAL` | `EncodeExternal` / `DecodeExternal` |

### Constraints

PER-encoding behavior depends on type constraints. The library models them as
first-class types so you can express the full X.691 constraint grammar:

- `IntegerConstraints` — unconstrained, semi-constrained, constrained, extensible,
  and big-integer (`math/big`) bounds.
- `SizeConstraints` — `SIZE(n)`, `SIZE(min..max)`, `SIZE(min..MAX)`, extensible.
- `EnumeratedConstraints` — root/extension enumeration value sets.
- `SequenceConstraints` — OPTIONAL/DEFAULT component info, extensions.
- `ChoiceConstraints` — root/extension alternatives and tag ordering.

Helper constructors: `Unconstrained()`, `Constrained()`, `SemiConstrained()`,
`ConstrainedExtensible()`, `ConstrainedBig()`, `FixedSize()`, `SizeRange()`,
`SizeMin()`.

## Installation

```bash
go get github.com/lvdund/asn1go
```

The package lives under `per/`:

```go
import "github.com/lvdund/asn1go/per"
```

## Quick Start

### Encoding

```go
package main

import (
    "fmt"

    "github.com/lvdund/asn1go/per"
)

func main() {
    // Use per.APER or per.UPER
    enc := per.NewEncoder(per.APER)

    // INTEGER (0..255)
    if err := enc.EncodeInteger(42, per.Constrained(0, 255)); err != nil {
        panic(err)
    }

    // BOOLEAN
    if err := enc.EncodeBoolean(true); err != nil {
        panic(err)
    }

    // OCTET STRING (SIZE 4)
    if err := enc.EncodeOctetString([]byte{0xDE, 0xAD, 0xBE, 0xEF}, per.FixedSize(4)); err != nil {
        panic(err)
    }

    fmt.Printf("%x\n", enc.Bytes()) // raw PER bytes
}
```

### Decoding

```go
dec := per.NewDecoder(data, per.APER)

i, err := dec.DecodeInteger(per.Constrained(0, 255))
if err != nil {
    panic(err)
}

b, err := dec.DecodeBoolean()
if err != nil {
    panic(err)
}

octets, err := dec.DecodeOctetString(per.FixedSize(4))
if err != nil {
    panic(err)
}
```

> **Important:** the variant passed to `NewDecoder` **must match** the variant used
> to encode the data (APER ↔ APER, UPER ↔ UPER). Mixing them produces invalid
> results because the alignment rules differ.

## Specifications

The encoder/decoder is implemented against:

- **ITU-T X.691** — ASN.1 Packed Encoding Rules (general PER).
- **3GPP TS 38.423 Annex A** — APER details used in 3GPP radio access networks.

See the `docs/` directory for spec notes, pseudocode, and reference material used
during implementation.

## Testing

The library ships with golden-vector and property tests for every ASN.1 type:

```bash
go test ./per/...
```

Golden tests verify encode/decode round-trips against known-good PER vectors,
and a variant-comparison test checks APER vs UPER alignment behavior.

## Design Notes

- **Bit-level first.** Encoding and decoding operate on an internal `BitStream`
  so unaligned UPER semantics are first-class, not an afterthought.
- **Constraints drive encoding.** Most type-encoding decisions (number of bits,
  length determinant form, extension bit) follow directly from the supplied
  constraints, mirroring X.691.
- **Self-contained.** Depends only on the Go standard library (+ `math/big`),
  so it is easy to vendor and audit.

## License

See the project repository for license information.

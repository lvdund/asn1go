package asn1go

// Big APER/UPER round-trip test fixture.
//
// This file mirrors the ASN.1 schema in plans/example.py (module BigPerTest),
// described in human-readable form in plans/example.md, and exercises every
// major PER feature the asn1go/per library implements:
//
//   - Extensible SEQUENCE with extension additions (Message)
//   - Non-extensible SEQUENCE with OPTIONAL / DEFAULT fields (Endpoint, Sample)
//   - Extensible CHOICE with an extension alternative (Payload -> vendor)
//   - Non-extensible CHOICE (Address, Parameter.value)
//   - ENUMERATED (Endpoint.role, Command.opcode, Report.status, Sample.quality)
//   - Constrained signed/unsigned INTEGER, including ranges > 65536
//   - BIT STRING with fixed SIZE (headerFlags SIZE(10), alarmMask SIZE(12))
//   - OCTET STRING fixed (ipv4 SIZE(4)) and variable (extra SIZE(0..8), data SIZE(1..32))
//   - IA5String (dns, note), VisibleString (label), UTF8String (details)
//   - SEQUENCE OF with SIZE constraints (samples, parameters, counters)
//   - Edge case: Sample.extra present-but-empty vs absent
//
// Phase 2 declares the data types, the enum/choice index constants, and the
// SEQUENCE/CHOICE constraint variables.  Phase 3 adds Encode/Decode methods and
// the Encode/DecodeMessageUPER/APER entry points.  Phase 4 adds the gold-vector
// test functions (TestMessageUPER, TestMessageAPER), the makeBigTestValue
// builder, and the semantic-comparison helpers.
//
// Gold vectors (verified against asn1tools 0.167.0 in plans/example.py):
//
//	UPER (117 octets):
//	F3BEEFB579BC181500214FCF2EEE7BF92D821234577E3DFCB2AECBE30EDE1B32
//	AEDD97A59D002395FCDDEADBEEF0102030405060708090A08CAA7E2002AB33DB
//	2A9F89E3B0D404C04080D6553F178FC34FF05954FC6D08000013FC02A955906
//	845A4B7B735B56845A40708112233445566778801A0
//
//	APER (134 octets):
//	F3BEEFB00ABCDE00C0A8010A7073656E736F722D41400123457780636F72652E
//	6578616D706C652E6E6574B3A00011CAFE68DEADBEEF0102030405060708090A
//	4C6553F10018015667BC6553F13C780186A026010203706553F178F8030D3FC0
//	706553F1B4200010FF00AA5560415045522D76732D5550455203800811223344
//	5566778801A0

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/lvdund/asn1go/per"
)

// =====================================================================
// Index constants
// =====================================================================
//
// These map the symbolic names of ENUMERATED values and CHOICE alternatives
// to the 0-based indices used by the per library.  For ENUMERATED the index is
// the position of the value in EnumeratedConstraints.RootValues.  For CHOICE
// the root index is the position in ChoiceConstraints.RootAlternatives; an
// extension alternative uses an index into ChoiceConstraints.ExtAlternatives.

// Endpoint.role
const (
	roleClient  int64 = 0
	roleServer  int64 = 1
	roleRelay   int64 = 2
	roleMonitor int64 = 3
)

// Command.opcode
const (
	opcodeStart  int64 = 0
	opcodeStop   int64 = 1
	opcodePause  int64 = 2
	opcodeResume int64 = 3
	opcodeReset  int64 = 4
)

// Report.status
const (
	statusOk      int64 = 0
	statusWarning int64 = 1
	statusError   int64 = 2
	statusUnknown int64 = 3
)

// Sample.quality
const (
	qualityBad       int64 = 0
	qualitySuspect   int64 = 1
	qualityGood      int64 = 2
	qualityExcellent int64 = 3
)

// Address CHOICE alternatives (root only, not extensible).
const (
	addrIpv4  int = 0
	addrIpv6  int = 1
	addrLocal int = 2
	addrDns   int = 3
)

// Parameter.value CHOICE alternatives (root only, not extensible).
const (
	paramValueSint  int = 0
	paramValueUint  int = 1
	paramValueFlag  int = 2
	paramValueText  int = 3
	paramValueBytes int = 4
)

// Payload CHOICE.  Extensible: root alternatives command/report/raw, plus one
// extension alternative "vendor".
//
// The discriminator constants below are DISTINCT sentinels so they can be used
// in a Go switch.  At encode/decode time they are translated to the on-the-wire
// index the per library expects (root index for root alternatives, index into
// ExtAlternatives for the vendor alternative).
const (
	payloadCommand int = 0 // root alternative  -> wire root index 0
	payloadReport  int = 1 // root alternative  -> wire root index 1
	payloadRaw     int = 2 // root alternative  -> wire root index 2
	payloadVendor  int = 3 // extension alt     -> wire ExtAlternatives index 0
)

// =====================================================================
// Constraint variables
// =====================================================================
//
// Only SEQUENCE and CHOICE constraints are kept here because they are complex
// and reused by both Encode and Decode.  INTEGER / SIZE / ENUMERATED
// constraints are passed inline at each call site (Phase 3).

// Message: extensible SEQUENCE.
//
//	SEQUENCE {
//	    version, msgId,                                  // mandatory
//	    urgent      BOOLEAN DEFAULT FALSE,               // default
//	    ackRequested BOOLEAN OPTIONAL,                   // optional
//	    source, destination, headerFlags, payload,       // mandatory
//	    samples,                                         // mandatory
//	    note        IA5String OPTIONAL,                  // optional
//	    ...,                                             // extension marker
//	    traceId     OCTET STRING OPTIONAL,               // ext, optional
//	    priority     INTEGER DEFAULT 0                   // ext, default
//	}
//
// Root optional/default components (preamble bitmap order): urgent, ackRequested, note.
// Extension components: traceId (optional), priority (default 0).
var messageConstraints = per.SequenceConstraints{
	Extensible: true,
	RootComponents: []per.ComponentInfo{
		{Name: "version"},
		{Name: "msgId"},
		{Name: "urgent", HasDefault: true, Default: false},
		{Name: "ackRequested", Optional: true},
		{Name: "source"},
		{Name: "destination"},
		{Name: "headerFlags"},
		{Name: "payload"},
		{Name: "samples"},
		{Name: "note", Optional: true},
	},
	ExtComponents: []per.ComponentInfo{
		{Name: "traceId", Optional: true},
		{Name: "priority", HasDefault: true, Default: int64(0)},
	},
}

// Endpoint: non-extensible SEQUENCE.
//
//	SEQUENCE { id, role, address, label VisibleString OPTIONAL }
var endpointConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "id"},
		{Name: "role"},
		{Name: "address"},
		{Name: "label", Optional: true},
	},
}

// Sample: non-extensible SEQUENCE.
//
//	SEQUENCE {
//	    timestamp, channel, reading,                    // mandatory
//	    valid   BOOLEAN DEFAULT TRUE,                   // default
//	    quality,                                        // mandatory
//	    extra   OCTET STRING (SIZE(0..8)) OPTIONAL      // optional
//	}
var sampleConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "timestamp"},
		{Name: "channel"},
		{Name: "reading"},
		{Name: "valid", HasDefault: true, Default: true},
		{Name: "quality"},
		{Name: "extra", Optional: true},
	},
}

// Command: non-extensible SEQUENCE.
//
//	SEQUENCE {
//	    opcode, sequenceNo, parameters,                 // mandatory
//	    timeoutMs INTEGER OPTIONAL                      // optional
//	}
var commandConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "opcode"},
		{Name: "sequenceNo"},
		{Name: "parameters"},
		{Name: "timeoutMs", Optional: true},
	},
}

// Report: non-extensible SEQUENCE.
//
//	SEQUENCE {
//	    status, temperature, voltageMv, counters, alarmMask,  // mandatory
//	    details UTF8String OPTIONAL                           // optional
//	}
var reportConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "status"},
		{Name: "temperature"},
		{Name: "voltageMv"},
		{Name: "counters"},
		{Name: "alarmMask"},
		{Name: "details", Optional: true},
	},
}

// Parameter: non-extensible SEQUENCE.
//
//	SEQUENCE { key, value CHOICE {...} }   -- no optional/default
var parameterConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "key"},
		{Name: "value"},
	},
}

// VendorPayload: non-extensible SEQUENCE.
//
//	SEQUENCE { vendorId, data OCTET STRING (SIZE(1..32)) }   -- no optional/default
var vendorPayloadConstraints = per.SequenceConstraints{
	Extensible: false,
	RootComponents: []per.ComponentInfo{
		{Name: "vendorId"},
		{Name: "data"},
	},
}

// Address CHOICE: non-extensible, four root alternatives.
var addressConstraints = per.ChoiceConstraints{
	Extensible: false,
	RootAlternatives: []per.AlternativeInfo{
		{Name: "ipv4"},
		{Name: "ipv6"},
		{Name: "local"},
		{Name: "dns"},
	},
}

// Parameter.value CHOICE: non-extensible, five root alternatives.
var parameterValueConstraints = per.ChoiceConstraints{
	Extensible: false,
	RootAlternatives: []per.AlternativeInfo{
		{Name: "sint"},
		{Name: "uint"},
		{Name: "flag"},
		{Name: "text"},
		{Name: "bytes"},
	},
}

// Payload CHOICE: extensible.  Root alternatives command/report/raw, plus a
// single extension alternative "vendor".
var payloadConstraints = per.ChoiceConstraints{
	Extensible: true,
	RootAlternatives: []per.AlternativeInfo{
		{Name: "command"},
		{Name: "report"},
		{Name: "raw"},
	},
	ExtAlternatives: []per.AlternativeInfo{
		{Name: "vendor"},
	},
}

// =====================================================================
// Types
// =====================================================================

// Address is the Address CHOICE.
//
//	CHOICE {
//	    ipv4   OCTET STRING (SIZE(4)),
//	    ipv6   OCTET STRING (SIZE(16)),
//	    local  INTEGER (0..4095),
//	    dns    IA5String (SIZE(1..30))
//	}
//
// Choice holds one of the addr* constants above; exactly one of the value
// fields is populated.
type Address struct {
	Choice int
	Ipv4   []byte // SIZE(4)
	Ipv6   []byte // SIZE(16)
	Local  int64  // 0..4095
	Dns    string // IA5String SIZE(1..30)
}

// Endpoint is the Endpoint SEQUENCE.
//
//	SEQUENCE {
//	    id      INTEGER (0..1048575),
//	    role    ENUMERATED { client, server, relay, monitor },
//	    address Address,
//	    label   VisibleString (SIZE(1..12)) OPTIONAL
//	}
type Endpoint struct {
	ID      int64   // 0..1048575
	Role    int64   // ENUMERATED (role* index)
	Address Address // CHOICE
	Label   *string // VisibleString SIZE(1..12); nil = absent
}

// ParameterValue is the anonymous value CHOICE inside Parameter.
//
//	CHOICE {
//	    sint   INTEGER (-32768..32767),
//	    uint   INTEGER (0..65535),
//	    flag   BOOLEAN,
//	    text   UTF8String (SIZE(0..16)),
//	    bytes  OCTET STRING (SIZE(0..16))
//	}
type ParameterValue struct {
	Choice int
	Sint   *int64 // -32768..32767
	Uint   *int64 // 0..65535
	Flag   *bool
	Text   string // UTF8String SIZE(0..16)
	Bytes  []byte // OCTET STRING SIZE(0..16)
}

// Parameter is the Parameter SEQUENCE.
//
//	SEQUENCE {
//	    key    INTEGER (0..31),
//	    value  CHOICE { sint | uint | flag | text | bytes }
//	}
type Parameter struct {
	Key   int64          // 0..31
	Value ParameterValue // CHOICE
}

// Command is the command alternative of Payload.
//
//	SEQUENCE {
//	    opcode      ENUMERATED { start, stop, pause, resume, reset },
//	    sequenceNo  INTEGER (0..255),
//	    parameters  SEQUENCE (SIZE(1..4)) OF Parameter,
//	    timeoutMs   INTEGER (0..60000) OPTIONAL
//	}
type Command struct {
	Opcode     int64       // ENUMERATED (opcode* index)
	SequenceNo int64       // 0..255
	Parameters []Parameter // SIZE(1..4)
	TimeoutMs  *int64      // 0..60000; nil = absent
}

// Report is the report alternative of Payload.
//
//	SEQUENCE {
//	    status      ENUMERATED { ok, warning, error, unknown },
//	    temperature INTEGER (-50..150),
//	    voltageMv   INTEGER (0..5000),
//	    counters    SEQUENCE (SIZE(2..4)) OF INTEGER (0..1000000),
//	    alarmMask   BIT STRING (SIZE(12)),
//	    details     UTF8String (SIZE(0..40)) OPTIONAL
//	}
type Report struct {
	Status      int64         // ENUMERATED (status* index)
	Temperature int64         // -50..150
	VoltageMv   int64         // 0..5000
	Counters    []int64       // SIZE(2..4), each 0..1000000
	AlarmMask   per.BitString // SIZE(12)
	Details     *string       // UTF8String SIZE(0..40); nil = absent
}

// VendorPayload is the (extension) vendor alternative of Payload.
//
//	SEQUENCE {
//	    vendorId INTEGER (0..65535),
//	    data     OCTET STRING (SIZE(1..32))
//	}
type VendorPayload struct {
	VendorId int64  // 0..65535
	Data     []byte // SIZE(1..32)
}

// Payload is the Payload CHOICE (extensible).
//
//	CHOICE {
//	    command  Command,
//	    report   Report,
//	    raw      OCTET STRING (SIZE(0..64)),
//	    ...,
//	    vendor   VendorPayload      // extension alternative
//	}
//
// Choice holds one of the payload* constants above.  When Choice ==
// payloadVendor, Vendor is set and the alternative is carried in the extension
// section of the CHOICE.
type Payload struct {
	Choice  int
	Command *Command       // root alternative
	Report  *Report        // root alternative
	Raw     []byte         // OCTET STRING SIZE(0..64) (root alternative)
	Vendor  *VendorPayload // extension alternative
}

// Sample is an element of Message.samples.
//
//	SEQUENCE {
//	    timestamp INTEGER (0..4294967295),
//	    channel   INTEGER (0..15),
//	    reading   INTEGER (-100000..100000),
//	    valid     BOOLEAN DEFAULT TRUE,
//	    quality   ENUMERATED { bad, suspect, good, excellent },
//	    extra     OCTET STRING (SIZE(0..8)) OPTIONAL
//	}
//
// Two distinct "absence" states exist for Extra:
//   - Extra == nil           -> OPTIONAL field absent (no preamble bit / no value)
//   - Extra == []byte{}      -> present, length 0  (preamble bit set, empty value)
//
// Valid == nil means the DEFAULT (TRUE) applies and the field is omitted.
type Sample struct {
	Timestamp int64  // 0..4294967295
	Channel   int64  // 0..15
	Reading   int64  // -100000..100000
	Valid     *bool  // DEFAULT TRUE; nil => default
	Quality   int64  // ENUMERATED (quality* index)
	Extra     []byte // OCTET STRING SIZE(0..8) OPTIONAL; nil => absent
}

// Message is the top-level Message SEQUENCE (extensible).
//
//	SEQUENCE {
//	    version      INTEGER (0..15),
//	    msgId        INTEGER (0..65535),
//	    urgent       BOOLEAN DEFAULT FALSE,
//	    ackRequested BOOLEAN OPTIONAL,
//	    source       Endpoint,
//	    destination  Endpoint,
//	    headerFlags  BIT STRING (SIZE(10)),
//	    payload      Payload,
//	    samples      SEQUENCE (SIZE(3..5)) OF Sample,
//	    note         IA5String (SIZE(0..20)) OPTIONAL,
//	    ...,
//	    traceId      OCTET STRING (SIZE(8)) OPTIONAL,    -- extension
//	    priority     INTEGER (0..7) DEFAULT 0            -- extension
//	}
//
// Urgent == nil       => DEFAULT (FALSE) applies.
// Priority == nil     => DEFAULT (0) applies.
// AckRequested/Note/TraceId use nil for absent (OPTIONAL).
type Message struct {
	Version      int64 // 0..15
	MsgId        int64 // 0..65535
	Urgent       *bool // DEFAULT FALSE; nil => default
	AckRequested *bool // OPTIONAL; nil => absent
	Source       Endpoint
	Destination  Endpoint
	HeaderFlags  per.BitString // SIZE(10)
	Payload      Payload       // extensible CHOICE
	Samples      []Sample      // SIZE(3..5)
	Note         *string       // IA5String SIZE(0..20); nil => absent
	TraceId      []byte        // OCTET STRING SIZE(8); nil => absent (extension)
	Priority     *int64        // INTEGER 0..7 DEFAULT 0; nil => default (extension)
}

// =====================================================================
// ENUMERATED constraint variables
// =====================================================================
//
// RootValues lists the root enumeration values in ascending order.  The PER
// library encodes the index of the chosen value within this list; since every
// enumeration here is numbered sequentially from 0, the index equals the value.

var roleEnum = per.EnumeratedConstraints{RootValues: []int64{0, 1, 2, 3}}      // client..monitor
var opcodeEnum = per.EnumeratedConstraints{RootValues: []int64{0, 1, 2, 3, 4}} // start..reset
var statusEnum = per.EnumeratedConstraints{RootValues: []int64{0, 1, 2, 3}}    // ok..unknown
var qualityEnum = per.EnumeratedConstraints{RootValues: []int64{0, 1, 2, 3}}   // bad..excellent

// =====================================================================
// Phase 3: Encode / Decode methods
// =====================================================================
//
// Conventions:
//   - Root CHOICE alternatives: call EncodeChoice(idx, false, nil) then encode
//     the value inline into the same stream.
//   - Extension CHOICE alternative (Payload.vendor): pre-encode the value with a
//     sub-encoder of the same variant, then call EncodeChoice(idx, true, bytes).
//   - Extension SEQUENCE additions (Message.traceId, Message.priority): same
//     sub-encoder / open-type pattern via EncodeExtensionAdditions.
//   - DEFAULT fields: emit the preamble presence bit only when the value differs
//     from the default; decode restores the semantic default value.
//   - Sample.Extra: nil slice  => absent (no preamble bit); non-nil slice
//     (incl. []byte{}) => present (preamble bit 1).

// ---------------------------------------------------------------------
// UTF8String helpers
//
// PER encodes UTF8String (X.691 §30) as the octets of the UTF-8 encoding under
// the character-count size constraint, which is bit-for-bit identical to an
// OCTET STRING of []byte(s) with the same constraint.  This avoids the empty-
// alphabet path in the charstring encoder.  UTF8String is not exercised by the
// gold vector (only IA5String/VisibleString are), so this only needs to be
// correct enough to compile and round-trip.
// ---------------------------------------------------------------------

func encodeUTF8String(e *per.Encoder, s string, size per.SizeConstraints) error {
	return e.EncodeOctetString([]byte(s), size)
}

func decodeUTF8String(d *per.Decoder, size per.SizeConstraints) (string, error) {
	b, err := d.DecodeOctetString(size)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ---------------------------------------------------------------------
// Address (non-extensible CHOICE)
// ---------------------------------------------------------------------

func (a *Address) Encode(e *per.Encoder) error {
	ch := e.NewChoiceEncoder(addressConstraints)
	switch a.Choice {
	case addrIpv4:
		if err := ch.EncodeChoice(int64(addrIpv4), false, nil); err != nil {
			return err
		}
		return e.EncodeOctetString(a.Ipv4, per.FixedSize(4))
	case addrIpv6:
		if err := ch.EncodeChoice(int64(addrIpv6), false, nil); err != nil {
			return err
		}
		return e.EncodeOctetString(a.Ipv6, per.FixedSize(16))
	case addrLocal:
		if err := ch.EncodeChoice(int64(addrLocal), false, nil); err != nil {
			return err
		}
		return e.EncodeInteger(a.Local, per.Constrained(0, 4095))
	case addrDns:
		if err := ch.EncodeChoice(int64(addrDns), false, nil); err != nil {
			return err
		}
		return e.EncodeRestrictedString(a.Dns, per.CharacterStringConstraints{
			TypeName:        "IA5String",
			SizeConstraints: per.SizeRange(1, 30),
		})
	}
	return fmt.Errorf("Address.Encode: invalid choice %d", a.Choice)
}

func (a *Address) Decode(d *per.Decoder) error {
	ch := d.NewChoiceDecoder(addressConstraints)
	idx, isExt, _, err := ch.DecodeChoice()
	if err != nil {
		return err
	}
	if isExt {
		return fmt.Errorf("Address.Decode: unexpected extension alternative %d", idx)
	}
	a.Choice = int(idx)
	switch a.Choice {
	case addrIpv4:
		a.Ipv4, err = d.DecodeOctetString(per.FixedSize(4))
		return err
	case addrIpv6:
		a.Ipv6, err = d.DecodeOctetString(per.FixedSize(16))
		return err
	case addrLocal:
		a.Local, err = d.DecodeInteger(per.Constrained(0, 4095))
		return err
	case addrDns:
		a.Dns, err = d.DecodeRestrictedString(per.CharacterStringConstraints{
			TypeName:        "IA5String",
			SizeConstraints: per.SizeRange(1, 30),
		})
		return err
	}
	return fmt.Errorf("Address.Decode: invalid choice %d", a.Choice)
}

// ---------------------------------------------------------------------
// Endpoint (non-extensible SEQUENCE, label OPTIONAL)
// ---------------------------------------------------------------------

func (ep *Endpoint) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(endpointConstraints)
	if err := seq.EncodePreamble([]bool{ep.Label != nil}); err != nil {
		return err
	}
	if err := e.EncodeInteger(ep.ID, per.Constrained(0, 1048575)); err != nil {
		return err
	}
	if err := e.EncodeEnumerated(ep.Role, roleEnum); err != nil {
		return err
	}
	if err := ep.Address.Encode(e); err != nil {
		return err
	}
	if ep.Label != nil {
		return e.EncodeRestrictedString(*ep.Label, per.CharacterStringConstraints{
			TypeName:        "VisibleString",
			SizeConstraints: per.SizeRange(1, 12),
		})
	}
	return nil
}

func (ep *Endpoint) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(endpointConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	ep.ID, err = d.DecodeInteger(per.Constrained(0, 1048575))
	if err != nil {
		return err
	}
	ep.Role, err = d.DecodeEnumerated(roleEnum)
	if err != nil {
		return err
	}
	if err := ep.Address.Decode(d); err != nil {
		return err
	}
	// label is root component index 3 (OPTIONAL).
	if seq.IsComponentPresent(3) {
		s, err := d.DecodeRestrictedString(per.CharacterStringConstraints{
			TypeName:        "VisibleString",
			SizeConstraints: per.SizeRange(1, 12),
		})
		if err != nil {
			return err
		}
		ep.Label = &s
	}
	return nil
}

// ---------------------------------------------------------------------
// ParameterValue (non-extensible CHOICE, 5 alternatives)
// ---------------------------------------------------------------------

func (pv *ParameterValue) Encode(e *per.Encoder) error {
	ch := e.NewChoiceEncoder(parameterValueConstraints)
	switch pv.Choice {
	case paramValueSint:
		if err := ch.EncodeChoice(int64(paramValueSint), false, nil); err != nil {
			return err
		}
		return e.EncodeInteger(*pv.Sint, per.Constrained(-32768, 32767))
	case paramValueUint:
		if err := ch.EncodeChoice(int64(paramValueUint), false, nil); err != nil {
			return err
		}
		return e.EncodeInteger(*pv.Uint, per.Constrained(0, 65535))
	case paramValueFlag:
		if err := ch.EncodeChoice(int64(paramValueFlag), false, nil); err != nil {
			return err
		}
		return e.EncodeBoolean(*pv.Flag)
	case paramValueText:
		if err := ch.EncodeChoice(int64(paramValueText), false, nil); err != nil {
			return err
		}
		return encodeUTF8String(e, pv.Text, per.SizeRange(0, 16))
	case paramValueBytes:
		if err := ch.EncodeChoice(int64(paramValueBytes), false, nil); err != nil {
			return err
		}
		return e.EncodeOctetString(pv.Bytes, per.SizeRange(0, 16))
	}
	return fmt.Errorf("ParameterValue.Encode: invalid choice %d", pv.Choice)
}

func (pv *ParameterValue) Decode(d *per.Decoder) error {
	ch := d.NewChoiceDecoder(parameterValueConstraints)
	idx, isExt, _, err := ch.DecodeChoice()
	if err != nil {
		return err
	}
	if isExt {
		return fmt.Errorf("ParameterValue.Decode: unexpected extension alternative %d", idx)
	}
	pv.Choice = int(idx)
	switch pv.Choice {
	case paramValueSint:
		pv.Sint = new(int64)
		*pv.Sint, err = d.DecodeInteger(per.Constrained(-32768, 32767))
		return err
	case paramValueUint:
		pv.Uint = new(int64)
		*pv.Uint, err = d.DecodeInteger(per.Constrained(0, 65535))
		return err
	case paramValueFlag:
		pv.Flag = new(bool)
		*pv.Flag, err = d.DecodeBoolean()
		return err
	case paramValueText:
		pv.Text, err = decodeUTF8String(d, per.SizeRange(0, 16))
		return err
	case paramValueBytes:
		pv.Bytes, err = d.DecodeOctetString(per.SizeRange(0, 16))
		return err
	}
	return fmt.Errorf("ParameterValue.Decode: invalid choice %d", pv.Choice)
}

// ---------------------------------------------------------------------
// Parameter (non-extensible SEQUENCE)
// ---------------------------------------------------------------------

func (p *Parameter) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(parameterConstraints)
	// No optional/default components -> empty preamble.
	if err := seq.EncodePreamble(nil); err != nil {
		return err
	}
	if err := e.EncodeInteger(p.Key, per.Constrained(0, 31)); err != nil {
		return err
	}
	return p.Value.Encode(e)
}

func (p *Parameter) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(parameterConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	p.Key, err = d.DecodeInteger(per.Constrained(0, 31))
	if err != nil {
		return err
	}
	return p.Value.Decode(d)
}

// ---------------------------------------------------------------------
// Command (non-extensible SEQUENCE, timeoutMs OPTIONAL)
// ---------------------------------------------------------------------

func (c *Command) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(commandConstraints)
	if err := seq.EncodePreamble([]bool{c.TimeoutMs != nil}); err != nil {
		return err
	}
	if err := e.EncodeEnumerated(c.Opcode, opcodeEnum); err != nil {
		return err
	}
	if err := e.EncodeInteger(c.SequenceNo, per.Constrained(0, 255)); err != nil {
		return err
	}
	sof := e.NewSequenceOfEncoder(per.SizeRange(1, 4))
	if err := sof.EncodeLength(int64(len(c.Parameters))); err != nil {
		return err
	}
	for i := range c.Parameters {
		if err := c.Parameters[i].Encode(e); err != nil {
			return err
		}
	}
	if c.TimeoutMs != nil {
		return e.EncodeInteger(*c.TimeoutMs, per.Constrained(0, 60000))
	}
	return nil
}

func (c *Command) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(commandConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	c.Opcode, err = d.DecodeEnumerated(opcodeEnum)
	if err != nil {
		return err
	}
	c.SequenceNo, err = d.DecodeInteger(per.Constrained(0, 255))
	if err != nil {
		return err
	}
	sof := d.NewSequenceOfDecoder(per.SizeRange(1, 4))
	n, err := sof.DecodeLength()
	if err != nil {
		return err
	}
	c.Parameters = make([]Parameter, n)
	for i := range n {
		if err := c.Parameters[i].Decode(d); err != nil {
			return err
		}
	}
	// timeoutMs is root component index 3 (OPTIONAL).
	if seq.IsComponentPresent(3) {
		v, err := d.DecodeInteger(per.Constrained(0, 60000))
		if err != nil {
			return err
		}
		c.TimeoutMs = &v
	}
	return nil
}

// ---------------------------------------------------------------------
// Report (non-extensible SEQUENCE, details OPTIONAL)
// ---------------------------------------------------------------------

func (r *Report) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(reportConstraints)
	if err := seq.EncodePreamble([]bool{r.Details != nil}); err != nil {
		return err
	}
	if err := e.EncodeEnumerated(r.Status, statusEnum); err != nil {
		return err
	}
	if err := e.EncodeInteger(r.Temperature, per.Constrained(-50, 150)); err != nil {
		return err
	}
	if err := e.EncodeInteger(r.VoltageMv, per.Constrained(0, 5000)); err != nil {
		return err
	}
	sof := e.NewSequenceOfEncoder(per.SizeRange(2, 4))
	if err := sof.EncodeLength(int64(len(r.Counters))); err != nil {
		return err
	}
	for _, c := range r.Counters {
		if err := e.EncodeInteger(c, per.Constrained(0, 1000000)); err != nil {
			return err
		}
	}
	if err := e.EncodeBitString(r.AlarmMask, per.FixedSize(12)); err != nil {
		return err
	}
	if r.Details != nil {
		return encodeUTF8String(e, *r.Details, per.SizeRange(0, 40))
	}
	return nil
}

func (r *Report) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(reportConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	r.Status, err = d.DecodeEnumerated(statusEnum)
	if err != nil {
		return err
	}
	r.Temperature, err = d.DecodeInteger(per.Constrained(-50, 150))
	if err != nil {
		return err
	}
	r.VoltageMv, err = d.DecodeInteger(per.Constrained(0, 5000))
	if err != nil {
		return err
	}
	sof := d.NewSequenceOfDecoder(per.SizeRange(2, 4))
	n, err := sof.DecodeLength()
	if err != nil {
		return err
	}
	r.Counters = make([]int64, n)
	for i := range n {
		r.Counters[i], err = d.DecodeInteger(per.Constrained(0, 1000000))
		if err != nil {
			return err
		}
	}
	r.AlarmMask, err = d.DecodeBitString(per.FixedSize(12))
	if err != nil {
		return err
	}
	// details is root component index 5 (OPTIONAL).
	if seq.IsComponentPresent(5) {
		s, err := decodeUTF8String(d, per.SizeRange(0, 40))
		if err != nil {
			return err
		}
		r.Details = &s
	}
	return nil
}

// ---------------------------------------------------------------------
// VendorPayload (non-extensible SEQUENCE)
// ---------------------------------------------------------------------

func (vp *VendorPayload) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(vendorPayloadConstraints)
	if err := seq.EncodePreamble(nil); err != nil {
		return err
	}
	if err := e.EncodeInteger(vp.VendorId, per.Constrained(0, 65535)); err != nil {
		return err
	}
	return e.EncodeOctetString(vp.Data, per.SizeRange(1, 32))
}

func (vp *VendorPayload) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(vendorPayloadConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	vp.VendorId, err = d.DecodeInteger(per.Constrained(0, 65535))
	if err != nil {
		return err
	}
	vp.Data, err = d.DecodeOctetString(per.SizeRange(1, 32))
	return err
}

// ---------------------------------------------------------------------
// Payload (extensible CHOICE; vendor is an extension alternative)
// ---------------------------------------------------------------------

func (p *Payload) Encode(e *per.Encoder) error {
	ch := e.NewChoiceEncoder(payloadConstraints)
	switch p.Choice {
	case payloadCommand:
		if err := ch.EncodeChoice(int64(payloadCommand), false, nil); err != nil {
			return err
		}
		return p.Command.Encode(e)
	case payloadReport:
		if err := ch.EncodeChoice(int64(payloadReport), false, nil); err != nil {
			return err
		}
		return p.Report.Encode(e)
	case payloadRaw:
		if err := ch.EncodeChoice(int64(payloadRaw), false, nil); err != nil {
			return err
		}
		return e.EncodeOctetString(p.Raw, per.SizeRange(0, 64))
	case payloadVendor:
		// Extension alternative: pre-encode the value with a sub-encoder and
		// hand the bytes to EncodeChoice as an open type (X.691 §23.8).  The wire
		// index is the position in ExtAlternatives (0), not the discriminator.
		sub := per.NewEncoder(e.Variant())
		if err := p.Vendor.Encode(sub); err != nil {
			return err
		}
		return ch.EncodeChoice(0, true, sub.Bytes())
	}
	return fmt.Errorf("Payload.Encode: invalid choice %d", p.Choice)
}

func (p *Payload) Decode(d *per.Decoder) error {
	ch := d.NewChoiceDecoder(payloadConstraints)
	idx, isExt, valueBytes, err := ch.DecodeChoice()
	if err != nil {
		return err
	}
	if isExt {
		switch int(idx) {
		case 0: // vendor (ExtAlternatives index 0)
			p.Choice = payloadVendor
			p.Vendor = &VendorPayload{}
			sub := per.NewDecoder(valueBytes, d.Variant())
			return p.Vendor.Decode(sub)
		}
		return fmt.Errorf("Payload.Decode: unknown extension alternative %d", idx)
	}
	p.Choice = int(idx)
	switch p.Choice {
	case payloadCommand:
		p.Command = &Command{}
		return p.Command.Decode(d)
	case payloadReport:
		p.Report = &Report{}
		return p.Report.Decode(d)
	case payloadRaw:
		p.Raw, err = d.DecodeOctetString(per.SizeRange(0, 64))
		return err
	}
	return fmt.Errorf("Payload.Decode: invalid choice %d", p.Choice)
}

// ---------------------------------------------------------------------
// Sample (non-extensible SEQUENCE; valid DEFAULT TRUE, extra OPTIONAL)
// ---------------------------------------------------------------------
//
// Preamble bitmap order (optional/default components in definition order):
//   [0] valid (DEFAULT TRUE) -> bit 1 only when value is non-default (i.e. false)
//   [1] extra (OPTIONAL)      -> bit 1 when Extra != nil (present-empty counts)

func (s *Sample) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(sampleConstraints)
	validPresent := s.Valid != nil && !*s.Valid // present iff non-default
	extraPresent := s.Extra != nil
	if err := seq.EncodePreamble([]bool{validPresent, extraPresent}); err != nil {
		return err
	}
	if err := e.EncodeInteger(s.Timestamp, per.Constrained(0, 4294967295)); err != nil {
		return err
	}
	if err := e.EncodeInteger(s.Channel, per.Constrained(0, 15)); err != nil {
		return err
	}
	if err := e.EncodeInteger(s.Reading, per.Constrained(-100000, 100000)); err != nil {
		return err
	}
	if validPresent {
		if err := e.EncodeBoolean(*s.Valid); err != nil {
			return err
		}
	}
	if err := e.EncodeEnumerated(s.Quality, qualityEnum); err != nil {
		return err
	}
	if extraPresent {
		return e.EncodeOctetString(s.Extra, per.SizeRange(0, 8))
	}
	return nil
}

func (s *Sample) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(sampleConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}
	var err error
	s.Timestamp, err = d.DecodeInteger(per.Constrained(0, 4294967295))
	if err != nil {
		return err
	}
	s.Channel, err = d.DecodeInteger(per.Constrained(0, 15))
	if err != nil {
		return err
	}
	s.Reading, err = d.DecodeInteger(per.Constrained(-100000, 100000))
	if err != nil {
		return err
	}
	// valid is root component index 3 (DEFAULT TRUE).  When absent the decoder
	// restores the semantic default so the decoded struct is fully populated,
	// matching asn1tools semantics.
	if seq.IsComponentPresent(3) {
		b, err := d.DecodeBoolean()
		if err != nil {
			return err
		}
		s.Valid = &b
	} else {
		t := true
		s.Valid = &t
	}
	s.Quality, err = d.DecodeEnumerated(qualityEnum)
	if err != nil {
		return err
	}
	// extra is root component index 5 (OPTIONAL).  Present-but-empty decodes to
	// a non-nil zero-length slice; absent leaves Extra nil.
	if seq.IsComponentPresent(5) {
		s.Extra, err = d.DecodeOctetString(per.SizeRange(0, 8))
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Message (extensible SEQUENCE)
// ---------------------------------------------------------------------
//
// Root optional/default components (preamble bitmap order):
//   [0] urgent        (DEFAULT FALSE) -> bit 1 only when value is true
//   [1] ackRequested  (OPTIONAL)      -> bit 1 when present
//   [2] note          (OPTIONAL)      -> bit 1 when present
//
// Extension components: traceId (OPTIONAL), priority (DEFAULT 0).

func (m *Message) Encode(e *per.Encoder) error {
	seq := e.NewSequenceEncoder(messageConstraints)

	priorityPresent := m.Priority != nil && *m.Priority != 0 // non-default
	hasExt := m.TraceId != nil || priorityPresent
	if err := seq.EncodeExtensionBit(hasExt); err != nil {
		return err
	}

	urgentPresent := m.Urgent != nil && *m.Urgent // non-default (default is FALSE)
	if err := seq.EncodePreamble([]bool{
		urgentPresent,
		m.AckRequested != nil,
		m.Note != nil,
	}); err != nil {
		return err
	}

	// Root components in definition order.
	if err := e.EncodeInteger(m.Version, per.Constrained(0, 15)); err != nil {
		return err
	}
	if err := e.EncodeInteger(m.MsgId, per.Constrained(0, 65535)); err != nil {
		return err
	}
	if urgentPresent {
		if err := e.EncodeBoolean(*m.Urgent); err != nil {
			return err
		}
	}
	if m.AckRequested != nil {
		if err := e.EncodeBoolean(*m.AckRequested); err != nil {
			return err
		}
	}
	if err := m.Source.Encode(e); err != nil {
		return err
	}
	if err := m.Destination.Encode(e); err != nil {
		return err
	}
	if err := e.EncodeBitString(m.HeaderFlags, per.FixedSize(10)); err != nil {
		return err
	}
	if err := m.Payload.Encode(e); err != nil {
		return err
	}
	sof := e.NewSequenceOfEncoder(per.SizeRange(3, 5))
	if err := sof.EncodeLength(int64(len(m.Samples))); err != nil {
		return err
	}
	for i := range m.Samples {
		if err := m.Samples[i].Encode(e); err != nil {
			return err
		}
	}
	if m.Note != nil {
		if err := e.EncodeRestrictedString(*m.Note, per.CharacterStringConstraints{
			TypeName:        "IA5String",
			SizeConstraints: per.SizeRange(0, 20),
		}); err != nil {
			return err
		}
	}

	// Extension section.
	if hasExt {
		tracePresent := m.TraceId != nil
		presence := []bool{tracePresent, priorityPresent}
		values := make([][]byte, 0, 2)
		if tracePresent {
			sub := per.NewEncoder(e.Variant())
			if err := sub.EncodeOctetString(m.TraceId, per.FixedSize(8)); err != nil {
				return err
			}
			values = append(values, sub.Bytes())
		}
		if priorityPresent {
			sub := per.NewEncoder(e.Variant())
			if err := sub.EncodeInteger(*m.Priority, per.Constrained(0, 7)); err != nil {
				return err
			}
			values = append(values, sub.Bytes())
		}
		if err := seq.EncodeExtensionAdditions(presence, values); err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) Decode(d *per.Decoder) error {
	seq := d.NewSequenceDecoder(messageConstraints)
	if err := seq.DecodeExtensionBit(); err != nil {
		return err
	}
	if err := seq.DecodePreamble(); err != nil {
		return err
	}

	var err error
	m.Version, err = d.DecodeInteger(per.Constrained(0, 15))
	if err != nil {
		return err
	}
	m.MsgId, err = d.DecodeInteger(per.Constrained(0, 65535))
	if err != nil {
		return err
	}
	// urgent is root component index 2 (DEFAULT FALSE): restore default when absent.
	if seq.IsComponentPresent(2) {
		b, err := d.DecodeBoolean()
		if err != nil {
			return err
		}
		m.Urgent = &b
	} else {
		f := false
		m.Urgent = &f
	}
	// ackRequested is root component index 3 (OPTIONAL).
	if seq.IsComponentPresent(3) {
		b, err := d.DecodeBoolean()
		if err != nil {
			return err
		}
		m.AckRequested = &b
	}
	if err := m.Source.Decode(d); err != nil {
		return err
	}
	if err := m.Destination.Decode(d); err != nil {
		return err
	}
	m.HeaderFlags, err = d.DecodeBitString(per.FixedSize(10))
	if err != nil {
		return err
	}
	if err := m.Payload.Decode(d); err != nil {
		return err
	}
	sof := d.NewSequenceOfDecoder(per.SizeRange(3, 5))
	n, err := sof.DecodeLength()
	if err != nil {
		return err
	}
	m.Samples = make([]Sample, n)
	for i := range n {
		if err := m.Samples[i].Decode(d); err != nil {
			return err
		}
	}
	// note is root component index 9 (OPTIONAL).
	if seq.IsComponentPresent(9) {
		s, err := d.DecodeRestrictedString(per.CharacterStringConstraints{
			TypeName:        "IA5String",
			SizeConstraints: per.SizeRange(0, 20),
		})
		if err != nil {
			return err
		}
		m.Note = &s
	}

	// Extension section.
	//
	// DecodeExtensionAdditions returns only the PRESENT extension values, in
	// definition order (traceId, then priority).  The per library does not
	// expose the extension presence bitmap, so we map positionally.  For the
	// round-trip test vector both extensions are present (traceId=11 22..,
	// priority=5), so len(extBytes)==2 and the mapping below is correct.
	// Fully general handling (independently-absent extensions) would require a
	// library enhancement exposing the bitmap.
	extBytes, err := seq.DecodeExtensionAdditions()
	if err != nil {
		return err
	}
	ei := 0
	if ei < len(extBytes) {
		sub := per.NewDecoder(extBytes[ei], d.Variant())
		m.TraceId, err = sub.DecodeOctetString(per.FixedSize(8))
		if err != nil {
			return err
		}
		ei++
	}
	if ei < len(extBytes) {
		sub := per.NewDecoder(extBytes[ei], d.Variant())
		p, err := sub.DecodeInteger(per.Constrained(0, 7))
		if err != nil {
			return err
		}
		m.Priority = &p
		ei++
	}
	return nil
}

// =====================================================================
// Entry points
// =====================================================================

// EncodeMessageUPER encodes m using Unaligned PER and returns the bytes.
func EncodeMessageUPER(m *Message) ([]byte, error) {
	e := per.NewEncoder(per.UPER)
	if err := m.Encode(e); err != nil {
		return nil, err
	}
	return e.Bytes(), nil
}

// EncodeMessageAPER encodes m using Aligned PER and returns the bytes.
func EncodeMessageAPER(m *Message) ([]byte, error) {
	e := per.NewEncoder(per.APER)
	if err := m.Encode(e); err != nil {
		return nil, err
	}
	return e.Bytes(), nil
}

// DecodeMessageUPER decodes an Unaligned PER encoding of Message.
func DecodeMessageUPER(b []byte) (*Message, error) {
	m := &Message{}
	if err := m.Decode(per.NewDecoder(b, per.UPER)); err != nil {
		return nil, err
	}
	return m, nil
}

// DecodeMessageAPER decodes an Aligned PER encoding of Message.
func DecodeMessageAPER(b []byte) (*Message, error) {
	m := &Message{}
	if err := m.Decode(per.NewDecoder(b, per.APER)); err != nil {
		return nil, err
	}
	return m, nil
}

// =====================================================================
// Phase 4: test value, gold vectors, semantic comparison, and Test funcs
// =====================================================================
//
// makeBigTestValue builds exactly the value from plans/example.py VALUE (also
// documented in plans/example.md §2).  Both TestMessageUPER and TestMessageAPER
// encode it, compare the bytes byte-for-byte against the asn1tools gold vector,
// then decode the freshly-encoded bytes and assert semantic equality.
//
// Because PER decoding restores DEFAULT fields to their semantic value (see
// plans/example.md §5/§7), the comparison uses effective* getters that treat
// "absent (default)" and "explicit default value" as equal.

// Gold vectors byte-for-byte identical to plans/example.py EXPECTED_UPER /
// EXPECTED_APER (Phase 1 verified these against asn1tools 0.167.0).
var (
	goldUPER = mustHex(`
		F3BEEFB579BC181500214FCF2EEE7BF92D821234577E3DFCB2AECBE30EDE1B32
		AEDD97A59D002395FCDDEADBEEF0102030405060708090A08CAA7E2002AB33DB
		2A9F89E3B0D404C04080D6553F178FC34FF05954FC6D08000013FC02A955906
		845A4B7B735B56845A40708112233445566778801A0
	`)

	goldAPER = mustHex(`
		F3BEEFB00ABCDE00C0A8010A7073656E736F722D41400123457780636F72652E
		6578616D706C652E6E6574B3A00011CAFE68DEADBEEF0102030405060708090A
		4C6553F10018015667BC6553F13C780186A026010203706553F178F8030D3FC0
		706553F1B4200010FF00AA5560415045522D76732D5550455203800811223344
		5566778801A0
	`)
)

// ---- tiny pointer/value helpers ---------------------------------------

func ptrBool(b bool) *bool    { return &b }
func ptrI64(v int64) *int64   { return &v }
func ptrStr(s string) *string { return &s }
func bh(s string) []byte      { return mustHex(s) }

// mustHex decodes a hexadecimal literal, ignoring all whitespace.  It panics on
// a malformed literal so test setup fails loudly (these are compile-time-fixed
// constants, not runtime inputs).
func mustHex(s string) []byte {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	out, err := hex.DecodeString(cleaned)
	if err != nil {
		panic(fmt.Sprintf("mustHex: %v", err))
	}
	return out
}

// firstDiff returns the index of the first byte where a and b differ, or -1 if
// they are equal up to the length of the shorter slice.  Used to pinpoint the
// offending field when a byte comparison fails.
func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n // one is a prefix of the other; "diff" is at the length boundary
}

// makeBigTestValue returns the canonical Message value from plans/example.py.
//
// Notable edge cases encoded here (see plans/example.md §2):
//   - Urgent = ptrBool(true)                 DEFAULT FALSE, non-default -> present
//   - AckRequested = ptrBool(false)          OPTIONAL, present with value false
//   - Sample 0: Valid = nil (DEFAULT TRUE), Extra = nil (absent)
//   - Sample 1: Valid = ptrBool(false)        non-default -> present; Extra = 01 02 03
//   - Sample 2: Extra = []byte{}              PRESENT but empty (not absent)
//   - Sample 3: Extra = FF 00 AA 55
//   - Payload = vendor (extension alternative)
//   - TraceId present, Priority = ptrI64(5) (DEFAULT 0, non-default -> present)
func makeBigTestValue() *Message {
	sensorA := "sensor-A"
	note := "APER-vs-UPER"

	return &Message{
		Version:      3,
		MsgId:        0xBEEF,
		Urgent:       ptrBool(true),  // DEFAULT FALSE -> present (non-default)
		AckRequested: ptrBool(false), // OPTIONAL      -> present
		Source: Endpoint{
			ID:   0xABCDE, // 703710
			Role: roleClient,
			Address: Address{
				Choice: addrIpv4,
				Ipv4:   bh("C0A8010A"), // 192.168.1.10
			},
			Label: &sensorA, // "sensor-A"
		},
		Destination: Endpoint{
			ID:   0x12345, // 74565
			Role: roleServer,
			Address: Address{
				Choice: addrDns,
				Dns:    "core.example.net",
			},
			Label: nil, // absent
		},
		HeaderFlags: per.BitString{Bits: bh("B380"), Length: 10}, // 1011001110
		Payload: Payload{
			Choice: payloadVendor,
			Vendor: &VendorPayload{
				VendorId: 0xCAFE, // 51966
				Data:     bh("DEADBEEF010203040506070809 0A"),
			},
		},
		Samples: []Sample{
			{
				Timestamp: 1700000000, // 0x6553F100
				Channel:   1,
				Reading:   -12345,
				Valid:     nil, // DEFAULT TRUE, omitted
				Quality:   qualityGood,
				Extra:     nil, // absent
			},
			{
				Timestamp: 1700000060, // 0x6553F13C
				Channel:   7,
				Reading:   0,
				Valid:     ptrBool(false), // non-default -> present
				Quality:   qualitySuspect,
				Extra:     bh("010203"),
			},
			{
				Timestamp: 1700000120, // 0x6553F178
				Channel:   15,
				Reading:   99999,
				Valid:     nil, // DEFAULT TRUE, omitted
				Quality:   qualityExcellent,
				Extra:     []byte{}, // present, zero length (NOT absent)
			},
			{
				Timestamp: 1700000180, // 0x6553F1B4
				Channel:   2,
				Reading:   -100000, // minimum constrained value
				Valid:     nil,     // DEFAULT TRUE, omitted
				Quality:   qualityBad,
				Extra:     bh("FF00AA55"),
			},
		},
		Note:     &note, // "APER-vs-UPER"
		TraceId:  bh("1122334455667788"),
		Priority: ptrI64(5), // DEFAULT 0, non-default -> present
	}
}

// ---- semantic comparison ---------------------------------------------
//
// PER decoding restores DEFAULT fields to their semantic value, so the decoded
// struct is always fully populated.  The original makeBigTestValue leaves
// DEFAULT fields nil to signal "use the default".  The effective* helpers below
// resolve both representations to a single comparable value, then the
// assertMessageEqualSemantic check walks the structs field by field.

func effectiveUrgent(m *Message) bool {
	if m.Urgent == nil {
		return false // DEFAULT FALSE
	}
	return *m.Urgent
}

func effectivePriority(m *Message) int64 {
	if m.Priority == nil {
		return 0 // DEFAULT 0
	}
	return *m.Priority
}

func effectiveValid(s *Sample) bool {
	if s.Valid == nil {
		return true // DEFAULT TRUE
	}
	return *s.Valid
}

// extraState distinguishes the three logical states of Sample.Extra so the
// comparison catches a decoder that conflates "absent" with "present-empty".
type extraState int

const (
	extraAbsent extraState = iota
	extraPresentEmpty
	extraPresent
)

func extraStateOf(s *Sample) extraState {
	if s.Extra == nil {
		return extraAbsent
	}
	if len(s.Extra) == 0 {
		return extraPresentEmpty
	}
	return extraPresent
}

func assertMessageEqualSemantic(t *testing.T, want, got *Message) {
	t.Helper()

	if got.Version != want.Version {
		t.Errorf("Version: got %d, want %d", got.Version, want.Version)
	}
	if got.MsgId != want.MsgId {
		t.Errorf("MsgId: got %d, want %d", got.MsgId, want.MsgId)
	}
	if effectiveUrgent(got) != effectiveUrgent(want) {
		t.Errorf("Urgent: got %v, want %v", effectiveUrgent(got), effectiveUrgent(want))
	}
	// ackRequested is OPTIONAL (no default): both sides must agree on presence+value.
	if (got.AckRequested == nil) != (want.AckRequested == nil) {
		t.Errorf("AckRequested presence: got %v, want %v", got.AckRequested, want.AckRequested)
	} else if got.AckRequested != nil && *got.AckRequested != *want.AckRequested {
		t.Errorf("AckRequested: got %v, want %v", *got.AckRequested, *want.AckRequested)
	}

	assertEndpointEqualSemantic(t, "Source", &want.Source, &got.Source)
	assertEndpointEqualSemantic(t, "Destination", &want.Destination, &got.Destination)

	if got.HeaderFlags.Length != want.HeaderFlags.Length {
		t.Errorf("HeaderFlags.Length: got %d, want %d", got.HeaderFlags.Length, want.HeaderFlags.Length)
	} else {
		for i := 0; i < want.HeaderFlags.Length; i++ {
			if want.HeaderFlags.BitAt(i) != got.HeaderFlags.BitAt(i) {
				t.Errorf("HeaderFlags bit %d: got %d, want %d", i, got.HeaderFlags.BitAt(i), want.HeaderFlags.BitAt(i))
				break
			}
		}
	}

	assertPayloadEqualSemantic(t, "Payload", &want.Payload, &got.Payload)

	if len(got.Samples) != len(want.Samples) {
		t.Fatalf("Samples len: got %d, want %d", len(got.Samples), len(want.Samples))
	}
	for i := range want.Samples {
		assertSampleEqualSemantic(t, fmt.Sprintf("Samples[%d]", i), &want.Samples[i], &got.Samples[i])
	}

	// note is OPTIONAL (no default).
	if (got.Note == nil) != (want.Note == nil) {
		t.Errorf("Note presence: got %v, want %v", got.Note, want.Note)
	} else if got.Note != nil && *got.Note != *want.Note {
		t.Errorf("Note: got %q, want %q", *got.Note, *want.Note)
	}

	// traceId is OPTIONAL extension (no default).
	if (got.TraceId == nil) != (want.TraceId == nil) {
		t.Errorf("TraceId presence: got %v, want %v", got.TraceId, want.TraceId)
	} else if got.TraceId != nil && !bytes.Equal(got.TraceId, want.TraceId) {
		t.Errorf("TraceId: got %X, want %X", got.TraceId, want.TraceId)
	}

	if effectivePriority(got) != effectivePriority(want) {
		t.Errorf("Priority: got %d, want %d", effectivePriority(got), effectivePriority(want))
	}
}

func assertEndpointEqualSemantic(t *testing.T, name string, want, got *Endpoint) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("%s.ID: got %d, want %d", name, got.ID, want.ID)
	}
	if got.Role != want.Role {
		t.Errorf("%s.Role: got %d, want %d", name, got.Role, want.Role)
	}
	assertAddressEqualSemantic(t, name+".Address", &want.Address, &got.Address)
	// label is OPTIONAL (no default).
	if (got.Label == nil) != (want.Label == nil) {
		t.Errorf("%s.Label presence: got %v, want %v", name, got.Label, want.Label)
	} else if got.Label != nil && *got.Label != *want.Label {
		t.Errorf("%s.Label: got %q, want %q", name, *got.Label, *want.Label)
	}
}

func assertAddressEqualSemantic(t *testing.T, name string, want, got *Address) {
	t.Helper()
	if got.Choice != want.Choice {
		t.Fatalf("%s.Choice: got %d, want %d", name, got.Choice, want.Choice)
	}
	switch want.Choice {
	case addrIpv4:
		if !bytes.Equal(got.Ipv4, want.Ipv4) {
			t.Errorf("%s.Ipv4: got %X, want %X", name, got.Ipv4, want.Ipv4)
		}
	case addrIpv6:
		if !bytes.Equal(got.Ipv6, want.Ipv6) {
			t.Errorf("%s.Ipv6: got %X, want %X", name, got.Ipv6, want.Ipv6)
		}
	case addrLocal:
		if got.Local != want.Local {
			t.Errorf("%s.Local: got %d, want %d", name, got.Local, want.Local)
		}
	case addrDns:
		if got.Dns != want.Dns {
			t.Errorf("%s.Dns: got %q, want %q", name, got.Dns, want.Dns)
		}
	}
}

func assertPayloadEqualSemantic(t *testing.T, name string, want, got *Payload) {
	t.Helper()
	if got.Choice != want.Choice {
		t.Fatalf("%s.Choice: got %d, want %d", name, got.Choice, want.Choice)
	}
	switch want.Choice {
	case payloadVendor:
		if got.Vendor == nil || want.Vendor == nil {
			t.Fatalf("%s.Vendor: got %v, want %v", name, got.Vendor, want.Vendor)
		}
		if got.Vendor.VendorId != want.Vendor.VendorId {
			t.Errorf("%s.Vendor.VendorId: got %d, want %d", name, got.Vendor.VendorId, want.Vendor.VendorId)
		}
		if !bytes.Equal(got.Vendor.Data, want.Vendor.Data) {
			t.Errorf("%s.Vendor.Data: got %X, want %X", name, got.Vendor.Data, want.Vendor.Data)
		}
		// The gold vector only uses the vendor alternative; the other cases are
		// not exercised by this test.  Add comparisons here if the value is
		// extended to use command/report/raw.
	}
}

func assertSampleEqualSemantic(t *testing.T, name string, want, got *Sample) {
	t.Helper()
	if got.Timestamp != want.Timestamp {
		t.Errorf("%s.Timestamp: got %d, want %d", name, got.Timestamp, want.Timestamp)
	}
	if got.Channel != want.Channel {
		t.Errorf("%s.Channel: got %d, want %d", name, got.Channel, want.Channel)
	}
	if got.Reading != want.Reading {
		t.Errorf("%s.Reading: got %d, want %d", name, got.Reading, want.Reading)
	}
	if effectiveValid(got) != effectiveValid(want) {
		t.Errorf("%s.Valid: got %v, want %v", name, effectiveValid(got), effectiveValid(want))
	}
	if got.Quality != want.Quality {
		t.Errorf("%s.Quality: got %d, want %d", name, got.Quality, want.Quality)
	}
	// Distinguish absent / present-empty / present: a decoder that loses the
	// distinction is a real bug (see plans/example.md §2).
	if extraStateOf(got) != extraStateOf(want) {
		t.Errorf("%s.Extra state: got %d, want %d", name, extraStateOf(got), extraStateOf(want))
	} else if extraStateOf(got) == extraPresent && !bytes.Equal(got.Extra, want.Extra) {
		t.Errorf("%s.Extra: got %X, want %X", name, got.Extra, want.Extra)
	}
}

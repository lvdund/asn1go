package per

import (
	"sort"
)

// SetEncoder provides methods for encoding SET structures.
// SET encoding per X.691 Clause 21 is identical to SEQUENCE encoding,
// except that root components MUST be encoded in canonical tag order.
type SetEncoder struct {
	sequenceEncoder *SequenceEncoder
	originalOrder   []int // Maps canonical order to original order
}

// SetDecoder provides methods for decoding SET structures.
type SetDecoder struct {
	sequenceDecoder *SequenceDecoder
	originalOrder   []int // Maps canonical order to original order
}

// SetConstraints defines SET constraints (identical to SequenceConstraints).
type SetConstraints = SequenceConstraints

// NewSetEncoder creates a new SET encoder with canonical tag ordering.
func (e *Encoder) NewSetEncoder(constraints SetConstraints) *SetEncoder {
	// Sort root components by canonical tag
	sortedConstraints, originalOrder := sortByCanonicalTag(constraints)

	return &SetEncoder{
		sequenceEncoder: e.NewSequenceEncoder(sortedConstraints),
		originalOrder:   originalOrder,
	}
}

// NewSetDecoder creates a new SET decoder with canonical tag ordering.
func (d *Decoder) NewSetDecoder(constraints SetConstraints) *SetDecoder {
	// Sort root components by canonical tag
	sortedConstraints, originalOrder := sortByCanonicalTag(constraints)

	return &SetDecoder{
		sequenceDecoder: d.NewSequenceDecoder(sortedConstraints),
		originalOrder:   originalOrder,
	}
}

// EncodeExtensionBit encodes the extension presence bit (if extensible).
func (se *SetEncoder) EncodeExtensionBit(hasExtensions bool) error {
	return se.sequenceEncoder.EncodeExtensionBit(hasExtensions)
}

// EncodePreamble encodes the preamble bitmap for optional/default root components.
// The bitmap is in canonical tag order, not definition order.
func (se *SetEncoder) EncodePreamble(presenceBitmap []bool) error {
	// presenceBitmap should already be in canonical order from caller
	return se.sequenceEncoder.EncodePreamble(presenceBitmap)
}

// EncodeExtensionAdditions encodes the extension section.
func (se *SetEncoder) EncodeExtensionAdditions(extensionPresence []bool, extensionValues [][]byte) error {
	return se.sequenceEncoder.EncodeExtensionAdditions(extensionPresence, extensionValues)
}

// DecodeExtensionBit decodes the extension presence bit (if extensible).
func (sd *SetDecoder) DecodeExtensionBit() error {
	return sd.sequenceDecoder.DecodeExtensionBit()
}

// DecodePreamble decodes the preamble bitmap for optional/default root components.
// The bitmap is in canonical tag order.
func (sd *SetDecoder) DecodePreamble() error {
	return sd.sequenceDecoder.DecodePreamble()
}

// IsComponentPresent returns whether a root component is present.
// componentIndex is in the ORIGINAL definition order (not canonical order).
func (sd *SetDecoder) IsComponentPresent(componentIndex int) bool {
	// Map original index to canonical index
	canonicalIndex := -1
	for i, orig := range sd.originalOrder {
		if orig == componentIndex {
			canonicalIndex = i
			break
		}
	}

	if canonicalIndex < 0 {
		return false
	}

	return sd.sequenceDecoder.IsComponentPresent(canonicalIndex)
}

// DecodeExtensionAdditions decodes the extension section.
func (sd *SetDecoder) DecodeExtensionAdditions() ([][]byte, error) {
	return sd.sequenceDecoder.DecodeExtensionAdditions()
}

// GetCanonicalOrder returns the mapping from original definition order to canonical order.
// This is useful for callers who need to encode/decode components in canonical order.
func (se *SetEncoder) GetCanonicalOrder() []int {
	return se.originalOrder
}

// GetCanonicalOrder returns the mapping from original definition order to canonical order.
func (sd *SetDecoder) GetCanonicalOrder() []int {
	return sd.originalOrder
}

// sortByCanonicalTag sorts SET components by canonical tag order per X.691 Clause 21.3.
// Returns the sorted constraints and a mapping from canonical order to original order.
func sortByCanonicalTag(constraints SetConstraints) (SetConstraints, []int) {
	// Create a slice of indices with tags for sorting
	type taggedComponent struct {
		component     ComponentInfo
		originalIndex int
	}

	rootComps := make([]taggedComponent, len(constraints.RootComponents))
	for i, comp := range constraints.RootComponents {
		rootComps[i] = taggedComponent{
			component:     comp,
			originalIndex: i,
		}
	}

	// Sort by tag (canonical order)
	sort.Slice(rootComps, func(i, j int) bool {
		return rootComps[i].component.Tag < rootComps[j].component.Tag
	})

	// Build sorted constraints and order mapping
	sortedComponents := make([]ComponentInfo, len(rootComps))
	originalOrder := make([]int, len(rootComps))
	for i, tc := range rootComps {
		sortedComponents[i] = tc.component
		originalOrder[i] = tc.originalIndex
	}

	sortedConstraints := SetConstraints{
		Extensible:     constraints.Extensible,
		RootComponents: sortedComponents,
		ExtComponents:  constraints.ExtComponents, // Extensions are not sorted
	}

	return sortedConstraints, originalOrder
}

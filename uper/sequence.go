package uper

import (
	"fmt"

	"github.com/lvdund/asn1go/utils"
)

func WriteSequenceOf[T UperMarshaller](items []T, uw *UperWriter, c *Constraint, e bool) (err error) {
	defer func() {
		err = utils.WrapError("WriteSequenceOf", err)
	}()

	numElems := len(items)

	var lowerBound, sizeRange uint64 = 0, 0
	if c != nil {
		if c.Lb < 0 || uint64(c.Lb) >= POW_16 {
			err = ErrConstraint
			return
		}
		lowerBound = uint64(c.Lb)
		sizeRange = c.Range()
		// UPER: no special handling for large upper bounds
	}

	if uint64(numElems) < lowerBound {
		err = ErrUnderflow
		return
	}

	if e {
		if sizeRange == 0 {
			err = ErrInextensible
			return
		}
		if err = uw.WriteBool(int64(numElems) > c.Ub); err != nil {
			return
		}
	}

	if sizeRange > 1 {
		if err = uw.writeConstraintValue(sizeRange, uint64(numElems)-lowerBound); err != nil {
			return
		}
	} else if sizeRange == 0 {
		// UPER: no alignment
		if err = uw.writeValue(uint64(numElems&0xff), 8); err != nil {
			return
		}
	}

	for _, item := range items {
		if err = item.Encode(uw); err != nil {
			return
		}
	}

	err = uw.flush()
	return
}

func ReadSequenceOf[T any](decoder func(ur *UperReader) (*T, error), ur *UperReader, c *Constraint, e bool) (items []T, err error) {
	var lowerBound, sizeRange uint64 = 0, 0
	if c != nil {
		if c.Lb < 0 || uint64(c.Lb) >= POW_16 {
			err = ErrConstraint
			return
		}
		lowerBound = uint64(c.Lb)
		sizeRange = c.Range()
	}

	var exBit bool
	if e {
		if sizeRange == 0 {
			err = ErrInextensible
			return
		}

		if exBit, err = ur.ReadBool(); err != nil {
			return
		}
	}

	var numElems uint64
	if sizeRange == 1 {
		numElems = lowerBound
	} else if sizeRange > 1 {
		if numElems, err = ur.readConstraintValue(sizeRange); err != nil {
			return
		}
		numElems += lowerBound
		if exBit && numElems <= uint64(c.Ub) {
			err = fmt.Errorf("Inconsistent extension bit")
			return
		}
	} else {
		// UPER: no alignment
		if numElems, err = ur.readValue(8); err != nil {
			return
		}
	}

	items = make([]T, numElems)
	var tmpItem *T
	for i := 0; i < int(numElems); i++ {
		if tmpItem, err = decoder(ur); err != nil {
			fmt.Println("\terr", err)
			return
		}
		items[i] = *tmpItem
	}
	return
}

func ReadSequenceOfEx[T UperUnmarshaller](fn func() T, ur *UperReader, c *Constraint, e bool) (items []T, err error) {
	decoder := func(ur *UperReader) (*T, error) {
		item := fn()
		if err := item.Decode(ur); err != nil {
			return nil, err
		}
		return &item, nil
	}
	items, err = ReadSequenceOf[T](decoder, ur, c, e)
	return
}

type ListContainer[T UperMarshaller] struct {
	list []T
	e    bool
	c    *Constraint
}

func NewListContainer[T UperMarshaller](list []T, c *Constraint, e bool) ListContainer[T] {
	return ListContainer[T]{
		list: list,
		e:    e,
		c:    c,
	}
}

func (l ListContainer[T]) Encode(uw *UperWriter) (err error) {
	err = WriteSequenceOf[T](l.list, uw, l.c, l.e)
	return
}

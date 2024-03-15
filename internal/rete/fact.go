package rete

import (
	"github.com/pkg/errors"
)

type (
	Cond struct {
		Alias     TVIdentity
		AliasAttr TVString
		Value     TestValue
		Negative  bool
		TestOp    TestOp

		AliasType *TypeInfo
	}
)

// TestOp
const (
	TestOpEqual TestOp = iota
	TestOpLess
	NTestOp
)

/*
 * Some constants to generate hash for Condition
 *
 * negative?  reserved          test_op_type(8)      name/attr/value hash(32)
 *    ^          ^                  ^                     ^
 *    |          |                  |                     |
 *   +-+------------------------+-------+------------------......--------------+
 * 63 62                     39     32 31                                    0
 */
const (
	condTestOpTypeOffset uint64 = 32
	condTestOpTypeMask   uint64 = ((1 << 8) - 1) << condTestOpTypeOffset
	condTestNegativeFlag uint64 = 1 << 63
)

const (
	CondHashOptMaskID uint64 = 1 << (iota)
	CondHashOptMaskValue
)

func (c Cond) Hash(opt uint64) uint64 {
	const condHashOptMaskIDAndValue = (CondHashOptMaskID | CondHashOptMaskValue)
	var x uint64 = 1
	if b := opt & condHashOptMaskIDAndValue; b == 0 {
		x = mix64(mix64(c.Alias.Hash(), c.AliasAttr.Hash()), c.Value.Hash())
	} else if b == condHashOptMaskIDAndValue {
		x = c.AliasAttr.Hash()
	} else if opt&CondHashOptMaskID != 0 {
		x = mix64(c.Value.Hash(), c.AliasAttr.Hash())
	} else if opt&CondHashOptMaskValue != 0 {
		x = mix64(c.Alias.Hash(), c.AliasAttr.Hash())
	}
	ret := ((uint64(c.TestOp) << condTestOpTypeOffset) & condTestOpTypeMask) | x
	if c.Negative {
		return ret | condTestNegativeFlag
	}
	return ret
}

type Fact struct {
	ID    TVIdentity
	Value TestValue
}

func (f Fact) WMEFromFact() *WME {
	return NewWME(f.ID, f.Value)
}

func (f Fact) Hash() uint64 {
	return mix64(f.ID.Hash(), f.Value.Hash())
}

func (f Fact) GetValue(field string) (TestValue, error) {
	if field == FieldSelf {
		return f.Value, nil
	}
	if f.Value.Type() != TestValueTypeStruct {
		return nil, errors.Errorf("cannot get field %s from %s", field, f.Value.Type())
	}
	return f.Value.(*TVStruct).GetField(field)
}

func (f Fact) HasField(field string) bool {
	if field == FieldSelf {
		return true
	}
	if f.Value.Type() != TestValueTypeStruct {
		panic(errors.Errorf("cannot get field %s from %s", field, f.Value.Type()))
	}
	return f.Value.(*TVStruct).HasField(field)
}

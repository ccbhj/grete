package rete

import (
	"fmt"

	. "github.com/ccbhj/grete/internal/types"
	"github.com/pkg/errors"
)

type (
	Cond struct {
		Alias     GVIdentity
		AliasAttr GVString
		Value     GValue
		Negative  bool
		TestOp    TestOp

		AliasType *TypeInfo
	}
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
	x = uint64(mix32(uint32(x), uint32(x>>32)))
	ret := ((uint64(c.TestOp) << condTestOpTypeOffset) & condTestOpTypeMask) | x
	if c.Negative {
		return ret | condTestNegativeFlag
	}
	return ret
}

type Fact struct {
	ID    GVIdentity
	Value GValue
}

func (f Fact) WMEFromFact() *WME {
	return NewWME(f.ID, f.Value)
}

func (f Fact) Hash() uint64 {
	return mix64(f.ID.Hash(), f.Value.Hash())
}

func (f Fact) GetValue(field string) (GValue, error) {
	if field == FieldSelf {
		return f.Value, nil
	}
	if f.Value.Type() != GValueTypeStruct {
		return nil, errors.Errorf("cannot get field %s from %s", field, f.Value.Type())
	}
	ret, _, err := f.Value.(*GVStruct).GetField(field)
	return ret, err
}

func (f Fact) HasField(field string) bool {
	if field == FieldSelf {
		return true
	}
	if f.Value.Type() != GValueTypeStruct {
		panic(errors.Errorf("cannot get field %s from %s", field, f.Value.Type()))
	}
	return f.Value.(*GVStruct).HasField(field)
}

type TestOp int

type TestFunc func(condValue, wmeValue GValue) bool

// TestOp
const (
	TestOpEqual TestOp = iota
	TestOpLess

	NTestOp
)

var testOp2Func = [NTestOp]TestFunc{
	TestOpEqual: TestEqual,
	TestOpLess:  TestLess,
}

func (t TestOp) ToFunc() TestFunc {
	return testOp2Func[t]
}

// TestFunc
func TestEqual(x, y GValue) bool {
	// TODO: check types of x and y
	return x.Equal(y)
}

func TestLess(l, r GValue) bool {
	if l.Type() != r.Type() {
		if x, ok := conv2Float(l); ok {
			if y, ok := conv2Float(r); ok {
				return x < y
			}
		}
		panic("cannot compare value with different type")
	}
	switch l.Type() {
	case GValueTypeInt:
		return l.(GVInt) < r.(GVInt)
	case GValueTypeUint:
		return l.(GVUint) < r.(GVUint)
	case GValueTypeFloat:
		return l.(GVFloat) < r.(GVFloat)
	}
	panic(fmt.Sprintf("less operator is unsupported for type %s", l.Type()))
}

func conv2Float(v GValue) (GVFloat, bool) {
	f, ok := v.(interface {
		toFloat() GVFloat
	})
	if !ok {
		return 0, false
	}
	return f.toFloat(), ok
}

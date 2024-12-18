package rete

import (
	"github.com/pkg/errors"

	. "github.com/ccbhj/grete/types"
)

type (
	Selector struct {
		Alias     GVIdentity
		AliasAttr GVString
	}

	// Guard define constant test on value
	Guard struct {
		AliasAttr GVString
		Value     GValue // should never be GVIdentity
		Negative  bool
		TestOp    TestOp
	}

	// JoinTest define join tests between two or more than two values
	JoinTest struct {
		Alias    []Selector
		TestOp   TestOp
		Negative bool
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

func (c Guard) Hash() uint64 {
	var x uint64
	x = mix64(c.Value.Hash(), c.AliasAttr.Hash())
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

type TestFunc func(...GValue) (bool, error)

// TestOp
const (
	TestOpEqual TestOp = iota
	TestOpLess

	NTestOp
)

func (t TestOp) String() string {
	switch t {
	case TestOpEqual:
		return "eq"
	case TestOpLess:
		return "less"
	}
	return "unknown"
}

var testOp2Func = [NTestOp]TestFunc{
	TestOpEqual: TestEqual,
	TestOpLess:  TestLess,
}

func (t TestOp) ToFunc() TestFunc {
	return testOp2Func[t]
}

////////////////////////////////////////////////////////////////////////////////////////////////
// Testing functions for TestOp
////////////////////////////////////////////////////////////////////////////////////////////////

func TestEqual(args ...GValue) (bool, error) {
	if len(args) < 2 {
		return false, errors.Errorf("TestOpEqual requires at least two args, but got %d", len(args))
	}
	x, y := args[0], args[1]
	// TODO: check types of x and y
	return x.Equal(y), nil
}

func TestLess(args ...GValue) (bool, error) {
	if len(args) < 2 {
		return false, errors.Errorf("TestOpLess requires at least two args, but got %d", len(args))
	}
	l, r := args[0], args[1]
	if l.Type() != r.Type() {
		if x, ok := conv2Float(l); ok {
			if y, ok := conv2Float(r); ok {
				return x < y, nil
			}
		}
		return false, errors.New("cannot compare value with different type")
	}
	switch l.Type() {
	case GValueTypeInt:
		return l.(GVInt) < r.(GVInt), nil
	case GValueTypeUint:
		return l.(GVUint) < r.(GVUint), nil
	case GValueTypeFloat:
		return l.(GVFloat) < r.(GVFloat), nil
	}
	return false, errors.Errorf("less operator is unsupported for type %s", l.Type())
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

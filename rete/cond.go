package rete

import (
	"fmt"
	"hash/fnv"
	"math"
)

type (
	TestValueType uint8

	TestValue interface {
		testValue()
		Hash() uint32
		Type() TestValueType
	}

	TestOp int

	Cond struct {
		Name     TVIdentity
		Attr     TVString
		Value    TestValue
		Negative bool
		TestOp   TestOp
	}
)

// TestValueType
const (
	TestValueTypeUnknown TestValueType = iota
	TestValueTypeIdentity
	TestValueTypeInt
	TestValueTypeUint
	TestValueTypeFloat
	TestValueTypeString
)

var testValueTypeDict = [...]string{
	"Unknown", "ID", "Int", "Uint", "Float", "String",
}

func (t TestValueType) String() string {
	if int(t) >= len(testValueTypeDict) {
		return testValueTypeDict[0]
	}

	return testValueTypeDict[t]
}

type TVIdentity string

func (TVIdentity) testValue()          {}
func (TVIdentity) Type() TestValueType { return TestValueTypeIdentity }
func (v TVIdentity) Hash() uint32 {
	f := fnv.New32()
	f.Write([]byte(v))
	return f.Sum32()
}

type TVString string

func (TVString) testValue()          {}
func (TVString) Type() TestValueType { return TestValueTypeString }
func (v TVString) Hash() uint32 {
	f := fnv.New32()
	f.Write([]byte(v))
	return f.Sum32()
}

type TVInt int64

func (TVInt) testValue()          {}
func (TVInt) Type() TestValueType { return TestValueTypeInt }
func (v TVInt) Hash() uint32      { return hash32(uint64(v)) }
func (v TVInt) toFloat() TVFloat  { return TVFloat(v) }

type TVUint uint64

func (TVUint) testValue()          {}
func (TVUint) Type() TestValueType { return TestValueTypeUint }
func (v TVUint) Hash() uint32      { return hash32(uint64(v)) }
func (v TVUint) toFloat() TVFloat  { return TVFloat(v) }

type TVFloat float64

func (TVFloat) testValue()          {}
func (TVFloat) Type() TestValueType { return TestValueTypeFloat }
func (v TVFloat) Hash() uint32      { return hash32(math.Float64bits(float64(v))) }
func (v TVFloat) toFloat() TVFloat  { return TVFloat(v) }

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

func (c Cond) Hash() uint64 {
	x := uint64(mix32(mix32(c.Name.Hash(), c.Attr.Hash()), c.Value.Hash()))
	ret := ((uint64(c.TestOp) << condTestOpTypeOffset) & condTestOpTypeMask) | x
	if c.Negative {
		return ret | condTestNegativeFlag
	}
	return ret
}

// TestOp
const (
	TestOpEqual TestOp = iota
	TestOpLess
	NTestOp
)

type TestFunc func(condValue, wmeValue TestValue) bool

var testOp2Func = [NTestOp]TestFunc{
	TestOpEqual: TestEqual,
	TestOpLess:  TestLess,
}

func (t TestOp) ToFunc() TestFunc {
	return testOp2Func[t]
}

// TestFunc
func TestEqual(x, y TestValue) bool {
	return x == y
}

func TestLess(l, r TestValue) bool {
	if l.Type() != r.Type() {
		if x, ok := conv2Float(l); ok {
			if y, ok := conv2Float(r); ok {
				return x < y
			}
		}
		panic("cannot compare value with different type")
	}
	switch l.Type() {
	case TestValueTypeInt:
		return l.(TVInt) < r.(TVInt)
	case TestValueTypeUint:
		return l.(TVUint) < r.(TVUint)
	case TestValueTypeFloat:
		return l.(TVFloat) < r.(TVFloat)
	}
	panic(fmt.Sprintf("less operator is unsupported for type %s", l.Type()))
}

func conv2Float(v TestValue) (TVFloat, bool) {
	f, ok := v.(interface {
		toFloat() TVFloat
	})
	if !ok {
		return 0, false
	}
	return f.toFloat(), ok
}

package rete

import (
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

	TestFunc func(condValue, wmeValue any) bool

	Cond struct {
		Name  TVIdentity
		Attr  TVString
		Value TestValue

		Negative bool
		testFn   TestFunc
	}
)

// TestValueType
const (
	TestValueTypeNone TestValueType = iota
	TestValueTypeIdentity
	TestValueTypeInt
	TestValueTypeUint
	TestValueTypeFloat
	TestValueTypeString
)

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
func (v TVInt) Hash() uint32 {
	return hash64(uint64(v))
}

type TVUint uint64

func (TVUint) testValue()          {}
func (TVUint) Type() TestValueType { return TestValueTypeUint }
func (v TVUint) Hash() uint32 {
	return hash64(uint64(v))
}

type TVFloat float64

func (TVFloat) testValue()          {}
func (TVFloat) Type() TestValueType { return TestValueTypeFloat }
func (v TVFloat) Hash() uint32 {
	return hash64(math.Float64bits(float64(v)))
}

// TestFunc

var TestEqual TestFunc = func(x, y any) bool {
	return x == y
}

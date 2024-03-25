package rete

import (
	"reflect"

	"github.com/dolthub/maphash"
	"github.com/pkg/errors"
)

const FieldSelf = "__Self__"
const FieldID = "__ID__"

var ErrFieldNotFound = errors.New("field not found")

type (
	TestValueType uint8

	TestValue interface {
		testValue()
		Hash() uint64
		Type() TestValueType
		RType() reflect.Type
		GetField(string) (TestValue, any, error)
		Equal(TestValue) bool
		ToGoValue() any
	}
)

// TestValueType
const (
	TestValueTypeUnknown TestValueType = iota
	TestValueTypeNil
	TestValueTypeIdentity
	TestValueTypeInt
	TestValueTypeUint
	TestValueTypeFloat
	TestValueTypeString
	TestValueTypeStruct
)

var testValueTypeDict = [...]string{
	"Unknown", "Nil", "ID", "Int", "Uint", "Float", "String",
}

var testValueTypeRTypeDict = [...]reflect.Type{
	nil, reflect.TypeOf(&TVNil{}), reflect.TypeOf(TVIdentity("")), reflect.TypeOf(TVInt(0)),
	reflect.TypeOf(TVUint(0)), reflect.TypeOf(TVFloat(0)),
	reflect.TypeOf(TVString("")),
}

func (t TestValueType) String() string {
	if int(t) >= len(testValueTypeDict) {
		return testValueTypeDict[0]
	}

	return testValueTypeDict[t]
}

var rType2testValueType = map[reflect.Type]TestValueType{
	reflect.TypeOf(&TVNil{}):       TestValueTypeNil,
	reflect.TypeOf(TVIdentity("")): TestValueTypeIdentity,
	reflect.TypeOf(TVInt(0)):       TestValueTypeInt,
	reflect.TypeOf(TVUint(0)):      TestValueTypeUint,
	reflect.TypeOf(TVFloat(0)):     TestValueTypeFloat,
	reflect.TypeOf(TVString("")):   TestValueTypeString,
	reflect.TypeOf(TVStruct{}):     TestValueTypeStruct,
}

func (t TestValueType) RType() reflect.Type {
	if int(t) >= len(testValueTypeRTypeDict) {
		return testValueTypeRTypeDict[0]
	}

	return testValueTypeRTypeDict[t]
}

type TVIdentity string

var tvIdentityHasher = maphash.NewHasher[TVIdentity]()

func (TVIdentity) testValue()            {}
func (TVIdentity) Type() TestValueType   { return TestValueTypeIdentity }
func (v TVIdentity) Hash() uint64        { return tvIdentityHasher.Hash(v) }
func (v TVIdentity) RType() reflect.Type { return reflect.TypeOf("") }
func (v TVIdentity) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return TVIdentity(string(v)), string(v), nil
}
func (v TVIdentity) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeIdentity && v == w.(TVIdentity)
}
func (v TVIdentity) ToGoValue() any { return string(v) }

type TVString string

var tvStringHasher = maphash.NewHasher[TVString]()

func (TVString) testValue()            {}
func (TVString) Type() TestValueType   { return TestValueTypeString }
func (v TVString) Hash() uint64        { return tvStringHasher.Hash(v) }
func (v TVString) RType() reflect.Type { return reflect.TypeOf("") }
func (v TVString) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeString && v == w.(TVString)
}
func (v TVString) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, string(v), nil
}
func (v TVString) ToGoValue() any { return v }

type TVInt int64

var tvIntHasher = maphash.NewHasher[TVInt]()

func (TVInt) testValue()            {}
func (TVInt) Type() TestValueType   { return TestValueTypeInt }
func (v TVInt) Hash() uint64        { return tvIntHasher.Hash(v) }
func (v TVInt) toFloat() TVFloat    { return TVFloat(v) }
func (v TVInt) RType() reflect.Type { return reflect.TypeOf(int64(0)) }
func (v TVInt) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeInt && v == w.(TVInt)
}
func (v TVInt) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, int64(v), nil
}
func (v TVInt) ToGoValue() any { return int64(v) }

type TVUint uint64

var tvUintHasher = maphash.NewHasher[TVUint]()

func (TVUint) testValue()            {}
func (TVUint) Type() TestValueType   { return TestValueTypeUint }
func (v TVUint) Hash() uint64        { return tvUintHasher.Hash(v) }
func (v TVUint) toFloat() TVFloat    { return TVFloat(v) }
func (v TVUint) RType() reflect.Type { return reflect.TypeOf(uint64(0)) }
func (v TVUint) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeUint && v == w.(TVUint)
}
func (v TVUint) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, uint64(v), nil
}
func (v TVUint) ToGoValue() any { return uint64(v) }

type TVFloat float64

var tvFloatHasher = maphash.NewHasher[TVFloat]()

func (TVFloat) testValue()            {}
func (TVFloat) Type() TestValueType   { return TestValueTypeFloat }
func (v TVFloat) Hash() uint64        { return tvFloatHasher.Hash(v) }
func (v TVFloat) toFloat() TVFloat    { return TVFloat(v) }
func (v TVFloat) RType() reflect.Type { return reflect.TypeOf(float64(0)) }
func (v TVFloat) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeFloat && v == w.(TVFloat)
}
func (v TVFloat) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, float64(v), nil
}
func (v TVFloat) ToGoValue() any { return float64(v) }

type TVNil struct {
}

func NewTVNil() *TVNil {
	return &TVNil{}
}

var tvNilHash = maphash.NewHasher[TVNil]().Hash(TVNil{})

func (*TVNil) testValue()               {}
func (*TVNil) Type() TestValueType      { return TestValueTypeNil }
func (v *TVNil) Hash() uint64           { return tvNilHash }
func (v *TVNil) RType() reflect.Type    { return reflect.TypeOf(&TVNil{}) }
func (v *TVNil) Equal(w TestValue) bool { return w.Type() == TestValueTypeNil }
func (v *TVNil) GetField(f string) (TestValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, nil, nil
}
func (v *TVNil) ToGoValue() any { return nil }

// type info of an Alias
type TypeInfo struct {
	T      TestValueType
	Fields map[string]TestValueType
	VT     reflect.Type
}

type TVStruct struct {
	v any // the actual struct
}

func NewTVStruct(v any) *TVStruct {
	return &TVStruct{v: v}
}

var tvStructHasher = maphash.NewHasher[TVStruct]()

func (TVStruct) testValue()            {}
func (TVStruct) Type() TestValueType   { return TestValueTypeStruct }
func (v TVStruct) Hash() uint64        { return tvStructHasher.Hash(v) }
func (v TVStruct) RType() reflect.Type { return reflect.TypeOf(v) }
func (v TVStruct) Value() any          { return v.v }
func (v TVStruct) ToGoValue() any      { return v.v }

// GetField extract field value by field name `f`, wrap it into TestValue and return it
func (v TVStruct) GetField(f string) (TestValue, any, error) {
	rv := reflect.Indirect(reflect.ValueOf(v.v))
	if !rv.IsValid() {
		return nil, nil, errors.New("nil value in TVStruct")
	}

	fv := rv.FieldByName(f)
	if !fv.IsValid() {
		return nil, nil, errors.WithMessagef(ErrFieldNotFound, "field=%s", f)
	}

	if _, in := rType2testValueType[fv.Type()]; in {
		ret := fv.Interface()
		return ret.(TestValue), ret, nil
	}

	isPtr := fv.Kind() == reflect.Pointer
	if isPtr && fv.IsZero() {
		return &TVNil{}, nil, nil
	}

	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TVInt(fv.Int()), fv.Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TVUint(fv.Uint()), fv.Interface(), nil
	case reflect.String:
		return TVString(fv.String()), fv.Interface(), nil
	case reflect.Float32, reflect.Float64:
		return TVFloat(fv.Float()), fv.Interface(), nil
	case reflect.Ptr, reflect.Struct:
		ret := fv.Interface()
		return &TVStruct{v: ret}, ret, nil
	default:
		return nil, nil, errors.Errorf("unsupported type of %s to get", f)
	}
}

func (v TVStruct) HasField(f string) bool {
	vt := reflect.TypeOf(v.v)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	_, in := vt.FieldByName(f)
	return in
}

func (v TVStruct) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeStruct &&
		v.v == w.(*TVStruct).v
}

func unwrapTestValue(v any) any {
	if v, ok := v.(TestValue); ok {
		return v.ToGoValue()
	}
	return v
}

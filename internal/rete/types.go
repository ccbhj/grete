package rete

import (
	"fmt"
	"reflect"

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
		GetField(string) (TestValue, error)
		Equal(TestValue) bool
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

func (t TestValueType) String() string {
	if int(t) >= len(testValueTypeDict) {
		return testValueTypeDict[0]
	}

	return testValueTypeDict[t]
}

var testValueTypeRTypeDict = [...]reflect.Type{
	nil, reflect.TypeOf(&TVNil{}), reflect.TypeOf(TVIdentity("")), reflect.TypeOf(TVInt(0)),
	reflect.TypeOf(TVUint(0)), reflect.TypeOf(TVFloat(0)),
	reflect.TypeOf(TVString("")),
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

func (TVIdentity) testValue()            {}
func (TVIdentity) Type() TestValueType   { return TestValueTypeIdentity }
func (v TVIdentity) Hash() uint64        { return hashAny(v) }
func (v TVIdentity) RType() reflect.Type { return reflect.TypeOf("") }
func (v TVIdentity) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return TVString(string(v)), nil
}
func (v TVIdentity) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeIdentity && v == w.(TVIdentity)
}

type TVString string

func (TVString) testValue()            {}
func (TVString) Type() TestValueType   { return TestValueTypeString }
func (v TVString) Hash() uint64        { return hashAny(v) }
func (v TVString) RType() reflect.Type { return reflect.TypeOf("") }
func (v TVString) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeString && v == w.(TVString)
}
func (v TVString) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return v, nil
}

type TVInt int64

func (TVInt) testValue()            {}
func (TVInt) Type() TestValueType   { return TestValueTypeInt }
func (v TVInt) Hash() uint64        { return hashAny(v) }
func (v TVInt) toFloat() TVFloat    { return TVFloat(v) }
func (v TVInt) RType() reflect.Type { return reflect.TypeOf(int64(0)) }
func (v TVInt) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeInt && v == w.(TVInt)
}
func (v TVInt) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return v, nil
}

type TVUint uint64

func (TVUint) testValue()            {}
func (TVUint) Type() TestValueType   { return TestValueTypeUint }
func (v TVUint) Hash() uint64        { return hashAny(v) }
func (v TVUint) toFloat() TVFloat    { return TVFloat(v) }
func (v TVUint) RType() reflect.Type { return reflect.TypeOf(uint64(0)) }
func (v TVUint) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeUint && v == w.(TVUint)
}
func (v TVUint) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return v, nil
}

type TVFloat float64

func (TVFloat) testValue()            {}
func (TVFloat) Type() TestValueType   { return TestValueTypeFloat }
func (v TVFloat) Hash() uint64        { return hashAny(v) }
func (v TVFloat) toFloat() TVFloat    { return TVFloat(v) }
func (v TVFloat) RType() reflect.Type { return reflect.TypeOf(float64(0)) }
func (v TVFloat) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeFloat && v == w.(TVFloat)
}
func (v TVFloat) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return v, nil
}

type TVNil struct {
}

func NewTVNil() *TVNil {
	return &TVNil{}
}

func (*TVNil) testValue()               {}
func (*TVNil) Type() TestValueType      { return TestValueTypeNil }
func (v *TVNil) Hash() uint64           { return hashAny(v) }
func (v *TVNil) RType() reflect.Type    { return reflect.TypeOf(&TVNil{}) }
func (v *TVNil) Equal(w TestValue) bool { return w.Type() == TestValueTypeNil }
func (v *TVNil) GetField(f string) (TestValue, error) {
	if f != FieldSelf {
		return nil, ErrFieldNotFound
	}
	return v, nil
}

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

func (*TVStruct) testValue()           {}
func (*TVStruct) Type() TestValueType  { return TestValueTypeStruct }
func (v *TVStruct) Hash() uint64       { return mix64(hashAny(v), hashAny(v.v)) }
func (v TVStruct) RType() reflect.Type { return reflect.TypeOf(v) }

func (v *TVStruct) Value() any { return v.v }

func (v *TVStruct) GetField(f string) (TestValue, error) {
	rv := reflect.Indirect(reflect.ValueOf(v.v))
	if !rv.IsValid() {
		return nil, errors.New("nil value in TVStruct")
	}

	fv := rv.FieldByName(f)
	if !fv.IsValid() {
		return nil, errors.WithMessagef(ErrFieldNotFound, "field=%s", f)
	}

	if _, in := rType2testValueType[fv.Type()]; in {
		return fv.Interface().(TestValue), nil
	}

	isPtr := fv.Kind() == reflect.Pointer
	if isPtr && fv.IsZero() {
		return &TVNil{}, nil
	}

	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TVInt(fv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TVUint(fv.Uint()), nil
	case reflect.String:
		return TVString(fv.String()), nil
	case reflect.Float32, reflect.Float64:
		return TVFloat(fv.Float()), nil
	case reflect.Ptr, reflect.Struct:
		return &TVStruct{v: fv.Interface()}, nil
	default:
		return nil, errors.Errorf("unsupported type of %s to get", f)
	}
}

func (v *TVStruct) HasField(f string) bool {
	vt := reflect.TypeOf(v.v)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	_, in := vt.FieldByName(f)
	return in
}

func (v *TVStruct) Equal(w TestValue) bool {
	return w != nil && w.Type() == TestValueTypeStruct &&
		v.v == w.(*TVStruct).v
}

type TestOp int

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
	// TODO: check types of x and y
	return x.Equal(y)
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
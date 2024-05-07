package types

import (
	"reflect"

	"github.com/dolthub/maphash"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/pkg/errors"
)

const FieldSelf = "__Self__"
const FieldID = "__ID__"

var ErrFieldNotFound = errors.New("field not found")

type (
	GValueType uint8

	GValue interface {
		testValue()
		Hash() uint64
		Type() GValueType
		RType() reflect.Type
		GetField(string) (GValue, any, error)
		Equal(GValue) bool
		ToGoValue() any
	}
)

// TestValueType
const (
	GValueTypeUnknown GValueType = iota
	GValueTypeNil
	GValueTypeIdentity
	GValueTypeInt
	GValueTypeUint
	GValueTypeFloat
	GValueTypeString
	GValueTypeStruct
	GValueTypeBool
)

var gValueTypeDict = [...]string{
	"Unknown", "Nil", "ID", "Int", "Uint", "Float", "String", "Bool",
}

var gValueTypeRTypeDict = [...]reflect.Type{
	nil, reflect.TypeOf(&GVNil{}), reflect.TypeOf(GVIdentity("")), reflect.TypeOf(GVInt(0)),
	reflect.TypeOf(GVUint(0)), reflect.TypeOf(GVFloat(0)),
	reflect.TypeOf(GVString("")), reflect.TypeOf(GVBool(false)),
}

func (t GValueType) String() string {
	if int(t) >= len(gValueTypeDict) {
		return gValueTypeDict[0]
	}

	return gValueTypeDict[t]
}

var rType2testValueType = map[reflect.Type]GValueType{
	reflect.TypeOf(&GVNil{}):       GValueTypeNil,
	reflect.TypeOf(GVIdentity("")): GValueTypeIdentity,
	reflect.TypeOf(GVInt(0)):       GValueTypeInt,
	reflect.TypeOf(GVUint(0)):      GValueTypeUint,
	reflect.TypeOf(GVFloat(0)):     GValueTypeFloat,
	reflect.TypeOf(GVString("")):   GValueTypeString,
	reflect.TypeOf(GVStruct{}):     GValueTypeStruct,
	reflect.TypeOf(GVBool(false)):  GValueTypeBool,
}

func (t GValueType) RType() reflect.Type {
	if int(t) >= len(gValueTypeRTypeDict) {
		return gValueTypeRTypeDict[0]
	}

	return gValueTypeRTypeDict[t]
}

type GVIdentity string

var tvIdentityHasher = maphash.NewHasher[GVIdentity]()

func (GVIdentity) testValue()            {}
func (GVIdentity) Type() GValueType      { return GValueTypeIdentity }
func (v GVIdentity) Hash() uint64        { return tvIdentityHasher.Hash(v) }
func (v GVIdentity) RType() reflect.Type { return reflect.TypeOf("") }
func (v GVIdentity) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return GVIdentity(string(v)), string(v), nil
}
func (v GVIdentity) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeIdentity && v == w.(GVIdentity)
}
func (v GVIdentity) ToGoValue() any { return string(v) }

type GVString string

var tvStringHasher = maphash.NewHasher[GVString]()

func (GVString) testValue()            {}
func (GVString) Type() GValueType      { return GValueTypeString }
func (v GVString) Hash() uint64        { return tvStringHasher.Hash(v) }
func (v GVString) RType() reflect.Type { return reflect.TypeOf("") }
func (v GVString) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeString && v == w.(GVString)
}
func (v GVString) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, string(v), nil
}
func (v GVString) ToGoValue() any { return v }

type GVInt int64

var tvIntHasher = maphash.NewHasher[GVInt]()

func (GVInt) testValue()            {}
func (GVInt) Type() GValueType      { return GValueTypeInt }
func (v GVInt) Hash() uint64        { return tvIntHasher.Hash(v) }
func (v GVInt) toFloat() GVFloat    { return GVFloat(v) }
func (v GVInt) RType() reflect.Type { return reflect.TypeOf(int64(0)) }
func (v GVInt) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeInt && v == w.(GVInt)
}
func (v GVInt) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, int64(v), nil
}
func (v GVInt) ToGoValue() any { return int64(v) }

type GVUint uint64

var tvUintHasher = maphash.NewHasher[GVUint]()

func (GVUint) testValue()            {}
func (GVUint) Type() GValueType      { return GValueTypeUint }
func (v GVUint) Hash() uint64        { return tvUintHasher.Hash(v) }
func (v GVUint) toFloat() GVFloat    { return GVFloat(v) }
func (v GVUint) RType() reflect.Type { return reflect.TypeOf(uint64(0)) }
func (v GVUint) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeUint && v == w.(GVUint)
}
func (v GVUint) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, uint64(v), nil
}
func (v GVUint) ToGoValue() any { return uint64(v) }

type GVFloat float64

var tvFloatHasher = maphash.NewHasher[GVFloat]()

func (GVFloat) testValue()            {}
func (GVFloat) Type() GValueType      { return GValueTypeFloat }
func (v GVFloat) Hash() uint64        { return tvFloatHasher.Hash(v) }
func (v GVFloat) toFloat() GVFloat    { return GVFloat(v) }
func (v GVFloat) RType() reflect.Type { return reflect.TypeOf(float64(0)) }
func (v GVFloat) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeFloat && v == w.(GVFloat)
}
func (v GVFloat) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, float64(v), nil
}
func (v GVFloat) ToGoValue() any { return float64(v) }

type GVBool bool

var tvBoolHasher = maphash.NewHasher[GVBool]()

func (GVBool) testValue()            {}
func (GVBool) Type() GValueType      { return GValueTypeBool }
func (v GVBool) Hash() uint64        { return tvBoolHasher.Hash(v) }
func (v GVBool) toBool() GVBool      { return GVBool(v) }
func (v GVBool) RType() reflect.Type { return reflect.TypeOf(float64(0)) }
func (v GVBool) Equal(w GValue) bool {
	return w.ToGoValue() == v.ToGoValue()
}
func (v GVBool) ToGoValue() any { return bool(v) }

func (v GVBool) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, bool(v), nil
}

type GVNil struct {
}

func NewGVNil() *GVNil {
	return &GVNil{}
}

var tvNilHash = maphash.NewHasher[GVNil]().Hash(GVNil{})

func (*GVNil) testValue()            {}
func (*GVNil) Type() GValueType      { return GValueTypeNil }
func (v *GVNil) Hash() uint64        { return tvNilHash }
func (v *GVNil) RType() reflect.Type { return reflect.TypeOf(&GVNil{}) }
func (v *GVNil) Equal(w GValue) bool { return w.Type() == GValueTypeNil }
func (v *GVNil) GetField(f string) (GValue, any, error) {
	if f != FieldSelf {
		return nil, nil, ErrFieldNotFound
	}
	return v, nil, nil
}
func (v *GVNil) ToGoValue() any { return nil }

// type info of an Alias
type TypeInfo struct {
	T      GValueType
	Fields map[string]GValueType
	VT     reflect.Type
}

func (v TypeInfo) Hash() uint64 {
	ret, err := hashstructure.Hash(v, hashstructure.FormatV2, nil)
	if err != nil {
		panic(err)
	}
	return ret
}

type GVStruct struct {
	V any // the actual struct
}

func NewGVStruct(v any) *GVStruct {
	return &GVStruct{V: v}
}

var tvStructHasher = maphash.NewHasher[GVStruct]()

func (GVStruct) testValue()            {}
func (GVStruct) Type() GValueType      { return GValueTypeStruct }
func (v GVStruct) Hash() uint64        { return tvStructHasher.Hash(v) }
func (v GVStruct) RType() reflect.Type { return reflect.TypeOf(v) }
func (v GVStruct) Value() any          { return v.V }
func (v GVStruct) ToGoValue() any      { return v.V }

// GetField extract field value by field name `f`, wrap it into TestValue and return it
func (v GVStruct) GetField(f string) (GValue, any, error) {
	rv := reflect.Indirect(reflect.ValueOf(v.V))
	if !rv.IsValid() {
		return nil, nil, errors.New("nil value in TVStruct")
	}

	fv := rv.FieldByName(f)
	if !fv.IsValid() {
		return nil, nil, errors.WithMessagef(ErrFieldNotFound, "field=%s", f)
	}

	if _, in := rType2testValueType[fv.Type()]; in {
		ret := fv.Interface()
		return ret.(GValue), ret, nil
	}

	isPtr := fv.Kind() == reflect.Pointer
	if isPtr && fv.IsZero() {
		return &GVNil{}, nil, nil
	}

	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return GVInt(fv.Int()), fv.Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return GVUint(fv.Uint()), fv.Interface(), nil
	case reflect.String:
		return GVString(fv.String()), fv.Interface(), nil
	case reflect.Float32, reflect.Float64:
		return GVFloat(fv.Float()), fv.Interface(), nil
	case reflect.Ptr, reflect.Struct:
		ret := fv.Interface()
		return &GVStruct{V: ret}, ret, nil
	default:
		return nil, nil, errors.Errorf("unsupported type of %s to get", f)
	}
}

func (v GVStruct) HasField(f string) bool {
	vt := reflect.TypeOf(v.V)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	_, in := vt.FieldByName(f)
	return in
}

func (v GVStruct) Equal(w GValue) bool {
	return w != nil && w.Type() == GValueTypeStruct &&
		v.V == w.(*GVStruct).V
}

func UnwrapTestValue(v any) any {
	if v, ok := v.(GValue); ok {
		return v.ToGoValue()
	}
	return v
}

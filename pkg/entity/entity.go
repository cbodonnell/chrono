package entity

// Entity is the universal schema-free entity.
type Entity struct {
	ID        string           `msgpack:"id"`
	Type      string           `msgpack:"type"`
	Timestamp int64            `msgpack:"ts"` // Unix nanoseconds — primary time axis
	Fields    map[string]Value `msgpack:"f"`
}

// ValueKind discriminates the type of a Value.
type ValueKind uint8

const (
	KindInt ValueKind = iota
	KindFloat
	KindBool
	KindString
	KindArray
	KindObject
)

// Value is a discriminated union covering all supported primitive types + arrays + objects.
type Value struct {
	Kind ValueKind        `msgpack:"k"`
	I    int64            `msgpack:"i,omitempty"`
	F    float64          `msgpack:"f,omitempty"`
	B    bool             `msgpack:"b,omitempty"`
	S    string           `msgpack:"s,omitempty"`
	Arr  []Value          `msgpack:"a,omitempty"`
	Obj  map[string]Value `msgpack:"o,omitempty"`
}

// NewInt creates an int64 Value.
func NewInt(i int64) Value {
	return Value{Kind: KindInt, I: i}
}

// NewFloat creates a float64 Value.
func NewFloat(f float64) Value {
	return Value{Kind: KindFloat, F: f}
}

// NewBool creates a bool Value.
func NewBool(b bool) Value {
	return Value{Kind: KindBool, B: b}
}

// NewString creates a string Value.
func NewString(s string) Value {
	return Value{Kind: KindString, S: s}
}

// NewArray creates an array Value from a slice of Values.
func NewArray(arr []Value) Value {
	return Value{Kind: KindArray, Arr: arr}
}

// NewStringArray creates an array Value from a slice of strings.
func NewStringArray(strs []string) Value {
	arr := make([]Value, len(strs))
	for i, s := range strs {
		arr[i] = NewString(s)
	}
	return Value{Kind: KindArray, Arr: arr}
}

// NewObject creates an object Value from a map of Values.
func NewObject(obj map[string]Value) Value {
	return Value{Kind: KindObject, Obj: obj}
}

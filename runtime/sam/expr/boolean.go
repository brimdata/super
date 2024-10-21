package expr

import (
	"bytes"
	"fmt"
	"math"
	"net/netip"
	"regexp"
	"regexp/syntax"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/zcode"
)

// Boolean is a function that takes a Value and returns a boolean result
// based on the typed value.
type Boolean func(super.Value) bool

var compareBool = map[string]func(bool, bool) bool{
	"==": func(a, b bool) bool { return a == b },
	"!=": func(a, b bool) bool { return a != b },
	">":  func(a, b bool) bool { return a && !b },
	">=": func(a, b bool) bool { return a || !b },
	"<":  func(a, b bool) bool { return !a && b },
	"<=": func(a, b bool) bool { return !a || b },
}

// CompareBool returns a Predicate that compares super.Values to a boolean literal
// that must be a boolean or coercible to an integer.  In the later case, the integer
// is converted to a boolean.
func CompareBool(op string, pattern bool) (Boolean, error) {
	compare, ok := compareBool[op]
	if !ok {
		return nil, fmt.Errorf("unknown bool comparator: %s", op)
	}
	return func(val super.Value) bool {
		if val.Type().ID() != super.IDBool {
			return false
		}
		b := val.Bool()
		return compare(b, pattern)
	}, nil
}

var compareInt = map[string]func(int64, int64) bool{
	"==": func(a, b int64) bool { return a == b },
	"!=": func(a, b int64) bool { return a != b },
	">":  func(a, b int64) bool { return a > b },
	">=": func(a, b int64) bool { return a >= b },
	"<":  func(a, b int64) bool { return a < b },
	"<=": func(a, b int64) bool { return a <= b }}

var compareFloat = map[string]func(float64, float64) bool{
	"==": func(a, b float64) bool { return a == b },
	"!=": func(a, b float64) bool { return a != b },
	">":  func(a, b float64) bool { return a > b },
	">=": func(a, b float64) bool { return a >= b },
	"<":  func(a, b float64) bool { return a < b },
	"<=": func(a, b float64) bool { return a <= b }}

// Return a predicate for comparing this value to one more typed
// byte slices by calling the predicate function with a Value.
// Operand is one of "==", "!=", "<", "<=", ">", ">=".
func CompareInt64(op string, pattern int64) (Boolean, error) {
	CompareInt, ok1 := compareInt[op]
	CompareFloat, ok2 := compareFloat[op]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("unknown int comparator: %s", op)
	}
	// many different Zed data types can be compared with integers
	return func(val super.Value) bool {
		switch val.Type().ID() {
		case super.IDUint8, super.IDUint16, super.IDUint32, super.IDUint64:
			if v := val.Uint(); v <= math.MaxInt64 {
				return CompareInt(int64(v), pattern)
			}
		case super.IDInt8, super.IDInt16, super.IDInt32, super.IDInt64, super.IDTime, super.IDDuration:
			return CompareInt(val.Int(), pattern)
		case super.IDFloat16, super.IDFloat32, super.IDFloat64:
			return CompareFloat(val.Float(), float64(pattern))
		}
		return false
	}, nil
}

// XXX should just do equality and we should compare in the encoded domain
// and not make copies and have separate cases for len 4 and len 16
var compareAddr = map[string]func(netip.Addr, netip.Addr) bool{
	"==": func(a, b netip.Addr) bool { return a.Compare(b) == 0 },
	"!=": func(a, b netip.Addr) bool { return a.Compare(b) != 0 },
	">":  func(a, b netip.Addr) bool { return a.Compare(b) > 0 },
	">=": func(a, b netip.Addr) bool { return a.Compare(b) >= 0 },
	"<":  func(a, b netip.Addr) bool { return a.Compare(b) < 0 },
	"<=": func(a, b netip.Addr) bool { return a.Compare(b) <= 0 },
}

// Comparison returns a Predicate that compares typed byte slices that must
// be TypeAddr with the value's address using a comparison based on op.
// Only equality operands are allowed.
func CompareIP(op string, pattern netip.Addr) (Boolean, error) {
	compare, ok := compareAddr[op]
	if !ok {
		return nil, fmt.Errorf("unknown addr comparator: %s", op)
	}
	return func(val super.Value) bool {
		if val.Type().ID() != super.IDIP {
			return false
		}
		return compare(super.DecodeIP(val.Bytes()), pattern)
	}, nil
}

// CompareFloat64 returns a Predicate that compares typed byte slices that must
// be coercible to an double with the value's double value using a comparison
// based on op.  Int, count, port, and double types can
// all be converted to the integer value.  XXX there are some overflow issues here.
func CompareFloat64(op string, pattern float64) (Boolean, error) {
	compare, ok := compareFloat[op]
	if !ok {
		return nil, fmt.Errorf("unknown double comparator: %s", op)
	}
	return func(val super.Value) bool {
		switch val.Type().ID() {
		// We allow comparison of float constant with integer-y
		// fields and just use typeDouble to parse since it will do
		// the right thing for integers.  XXX do we want to allow
		// integers that cause float64 overflow?  user can always
		// use an integer constant instead of a float constant to
		// compare with the integer-y field.
		case super.IDUint8, super.IDUint16, super.IDUint32, super.IDUint64:
			return compare(float64(val.Uint()), pattern)
		case super.IDInt8, super.IDInt16, super.IDInt32, super.IDInt64, super.IDTime, super.IDDuration:
			return compare(float64(val.Int()), pattern)
		case super.IDFloat16, super.IDFloat32, super.IDFloat64:
			return compare(val.Float(), pattern)
		}
		return false
	}, nil
}

var compareString = map[string]func(string, string) bool{
	"==": func(a, b string) bool { return a == b },
	"!=": func(a, b string) bool { return a != b },
	">":  func(a, b string) bool { return a > b },
	">=": func(a, b string) bool { return a >= b },
	"<":  func(a, b string) bool { return a < b },
	"<=": func(a, b string) bool { return a <= b },
}

func CompareString(op string, pattern []byte) (Boolean, error) {
	compare, ok := compareString[op]
	if !ok {
		return nil, fmt.Errorf("unknown string comparator: %s", op)
	}
	s := string(pattern)
	return func(val super.Value) bool {
		if val.Type().ID() == super.IDString {
			return compare(byteconv.UnsafeString(val.Bytes()), s)
		}
		return false
	}, nil
}

var compareBytes = map[string]func([]byte, []byte) bool{
	"==": func(a, b []byte) bool { return bytes.Equal(a, b) },
	"!=": func(a, b []byte) bool { return !bytes.Equal(a, b) },
	">":  func(a, b []byte) bool { return bytes.Compare(a, b) > 0 },
	">=": func(a, b []byte) bool { return bytes.Compare(a, b) >= 0 },
	"<":  func(a, b []byte) bool { return bytes.Compare(a, b) < 0 },
	"<=": func(a, b []byte) bool { return bytes.Compare(a, b) <= 0 },
}

func CompareBytes(op string, pattern []byte) (Boolean, error) {
	compare, ok := compareBytes[op]
	if !ok {
		return nil, fmt.Errorf("unknown bytes comparator: %s", op)
	}
	return func(val super.Value) bool {
		switch val.Type().ID() {
		case super.IDBytes, super.IDType:
			return compare(val.Bytes(), pattern)
		}
		return false
	}, nil
}

func CompileRegexp(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		if syntaxErr, ok := err.(*syntax.Error); ok {
			syntaxErr.Expr = pattern
		}
		return nil, err
	}
	return re, err
}

// NewRegexpBoolean returns a Boolean that compares values that must
// be a stringy the given regexp.
func NewRegexpBoolean(re *regexp.Regexp) Boolean {
	return func(val super.Value) bool {
		if val.IsString() {
			return re.Match(val.Bytes())
		}
		return false
	}
}

func CompareNull(op string) (Boolean, error) {
	switch op {
	case "==":
		return func(val super.Value) bool {
			return val.IsNull()
		}, nil
	case "!=":
		return func(val super.Value) bool {
			return !val.IsNull()
		}, nil
	default:
		return nil, fmt.Errorf("unknown null comparator: %s", op)
	}
}

// Given a predicate for comparing individual elements, produce a new
// predicate that implements the "in" comparison.
func Contains(compare Boolean) Boolean {
	return func(val super.Value) bool {
		return errMatch == val.Walk(func(typ super.Type, body zcode.Bytes) error {
			if compare(super.NewValue(typ, body)) {
				return errMatch
			}
			return nil
		})
	}
}

// Comparison returns a Predicate for comparing this value to other values.
// The op argument is one of "==", "!=", "<", "<=", ">", ">=".
// See the comments of the various type implementations
// of this method as some types limit the operand to equality and
// the various types handle coercion in different ways.
func Comparison(op string, val super.Value) (Boolean, error) {
	switch super.TypeUnder(val.Type()).(type) {
	case *super.TypeOfNull:
		return CompareNull(op)
	case *super.TypeOfIP:
		return CompareIP(op, super.DecodeIP(val.Bytes()))
	case *super.TypeOfBool:
		return CompareBool(op, val.Bool())
	case *super.TypeOfFloat64:
		return CompareFloat64(op, val.Float())
	case *super.TypeOfString:
		return CompareString(op, val.Bytes())
	case *super.TypeOfBytes, *super.TypeOfType:
		return CompareBytes(op, val.Bytes())
	case *super.TypeOfInt64, *super.TypeOfTime, *super.TypeOfDuration:
		return CompareInt64(op, super.DecodeInt(val.Bytes()))
	default:
		return nil, fmt.Errorf("literal comparison of type %q unsupported", val.Type())
	}
}

package expr

import (
	"errors"
	"fmt"
	"math"

	"github.com/brimsec/zq/zng"
)

type Function func([]NativeValue) (NativeValue, error)

var ErrWrongArgc = errors.New("wrong number of arguments")
var ErrBadArgument = errors.New("bad argument")

var allFns = []struct {
	name string
	fn   Function
}{
	{"Math.sqrt", mathSqrt},
}

var allFnsMap map[string]Function
var fnsInited = false

func lookupFunction(name string) *Function {
	if !fnsInited {
		allFnsMap = make(map[string]Function)
		for _, f := range allFns {
			allFnsMap[f.name] = f.fn
		}
	}
	fn, ok := allFnsMap[name]
	if ok {
		return &fn
	}
	return nil
}

func mathSqrt(args []NativeValue) (NativeValue, error) {
	if len(args) < 1 || len(args) > 1 {
		return NativeValue{}, fmt.Errorf("Math.sqrt: %w", ErrWrongArgc)
	}

	var x float64
	switch args[0].typ.ID() {
	case zng.IdFloat64:
		x = args[0].value.(float64)
	case zng.IdInt16, zng.IdInt32, zng.IdInt64:
		x = float64(args[0].value.(int64))
	case zng.IdByte, zng.IdUint16, zng.IdUint32, zng.IdUint64:
		x = float64(args[0].value.(uint64))
	default:
		return NativeValue{}, fmt.Errorf("Math.sqrt: %w", ErrBadArgument)
	}

	r := math.Sqrt(x)
	if math.IsNaN(r) {
		return NativeValue{}, fmt.Errorf("Math.sqrt: %w", ErrBadArgument)
	}

	return NativeValue{zng.TypeFloat64, r}, nil
}

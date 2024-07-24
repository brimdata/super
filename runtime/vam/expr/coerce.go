package expr

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/byteconv"
	"github.com/brimdata/zed/runtime/sam/expr/coerce"
	"github.com/brimdata/zed/vector"
)

// coerceVals checks if a and b are type compatible for comparison
// and/or math and modifies one of the vectors promoting to the
// other's type as needed according to Zed's coercion rules (modified
// vectors are returned, not changed in place).  When errors are
// encountered an error vector is returned and the coerced values
// are abandoned.
func coerceVals(zctx *zed.Context, a, b vector.Any) (vector.Any, vector.Any, vector.Any) {
	aid := a.Type().ID()
	bid := b.Type().ID()
	if aid == bid {
		//XXX this catches complex types so we need to add support
		// for things like {a:10}<{a:123} or [1,2]+[3,4]
		// sam doesn't support this yet.
		return a, b, nil
	}
	if aid == zed.IDNull {
		return a, b, nil //XXX
	}
	if bid == zed.IDNull {
		return a, b, nil //XXX
	}
	if !zed.IsNumber(aid) || !zed.IsNumber(bid) {
		return nil, nil, vector.NewStringError(zctx, coerce.ErrIncompatibleTypes.Error(), a.Len())
	}
	// Both a and b are numbers.  We need to promote to a common
	// type based on Zed's coercion rules.
	// XXX currently vector supports only 64-bit stuff...
	// need to handle all sizes.
	if zed.IsFloat(aid) {
		//if aid == zed.IDFloat16 {
		//	c.A = c.buf2.Float64(float64(zed.DecodeFloat16(c.A)))
		//} else if aid == zed.IDFloat32 {
		//	c.A = c.buf2.Float64(float64(zed.DecodeFloat32(c.A)))
		//}
		// need to handle other number types not just ints
		return a, intToFloat(b), nil
	}
	if zed.IsFloat(bid) {
		//if bid == zed.IDFloat16 {
		//	c.B = c.buf2.Float64(float64(zed.DecodeFloat16(c.B)))
		//} else if bid == zed.IDFloat32 {
		//	c.B = c.buf2.Float64(float64(zed.DecodeFloat32(c.B)))
		//}
		return intToFloat(a), b, nil
	}
	aIsSigned := zed.IsSigned(aid)
	if aIsSigned == zed.IsSigned(bid) {
		// They have the same signed-ness.  Promote to the wider
		// type by rank and leave the zcode.Bytes as is since
		// the varint encoding is the same for all the widths.
		// Width increasese with type ID.
		id := aid
		if bid > id {
			id = bid
		}
		return promoteWider(id, a), promoteWider(id, b), nil
	}
	// Need to handle other uint64 types  dur and time

	//id := coerce.PromoteInt(aid, bid)
	// Otherwise, we'll promote mixed signed-ness to signed unless
	// the unsigned value is greater than signed maxint, in which
	// case, we report an overflow error.
	if aIsSigned {
		//XXX overflow errors
		return a, promoteToSigned(b), nil
	} else {
		return promoteToSigned(a), b, nil
	}
	//if !ok {
	// We got overflow trying to turn the unsigned to signed,
	// so try turning the signed into unsigned.
	//	if aIsSigned {
	//		c.A, ok = c.promoteToUnsigned(c.A)
	//	} else {
	//		c.B, ok = c.promoteToUnsigned(c.B)
	//	}
	//	id = zed.IDUint64
	//}
	//return id, ok
}

//XXX need to handle const here

func promoteWider(id int, val vector.Any) vector.Any {
	typ, err := zed.LookupPrimitiveByID(id)
	if err != nil {
		panic(err)
	}
	switch val := val.(type) {
	case *vector.Int:
		return val.Promote(typ)
	case *vector.Uint:
		return val.Promote(typ)
	case *vector.Dict:
		promoted := val.Any.(vector.Promotable).Promote(typ)
		return vector.NewDict(promoted, val.Index, val.Counts, val.Nulls)
	case *vector.View:
		promoted := val.Any.(vector.Promotable).Promote(typ)
		return vector.NewView(val.Index, promoted)
	default:
		panic(fmt.Sprintf("promoteWider %T", val))
	}
}

// XXX need to handle overflow errors
func promoteToSigned(val vector.Any) vector.Any {
	//XXX need wide variant here if we're going to support this semantic
	//if v > math.MaxInt64 {
	//	return nil, false
	//}
	switch val := val.(type) {
	case *vector.Int:
		return val
	case *vector.Uint:
		return uintToInt(val)
	case *vector.Const:
		v, ok := ToInt(val.Value())
		if !ok {
			panic("ToInt failed")
		}
		return vector.NewConst(nil, zed.NewInt64(v), val.Len(), val.Nulls)
	case *vector.Dict:
		promoted := promoteToSigned(val.Any)
		return vector.NewDict(promoted, val.Index, val.Counts, val.Nulls)
	case *vector.View:
		promoted := promoteToSigned(val.Any)
		return vector.NewView(val.Index, promoted)
	default:
		panic(fmt.Sprintf("promoteToSigned %T", val))
	}
}

//func (c *Pair) promoteToUnsigned(in zcode.Bytes) (zcode.Bytes, bool) {
//	v := zed.DecodeInt(in)
//	if v < 0 {
//		return nil, false
//	}
//	return c.Uint(uint64(v)), true
//}

func ToFloat(val zed.Value) (float64, bool) {
	switch id := val.Type().ID(); {
	case zed.IsUnsigned(id):
		return float64(val.Uint()), true
	case zed.IsSigned(id):
		return float64(val.Int()), true
	case zed.IsFloat(id):
		return val.Float(), true
	case id == zed.IDString:
		v, err := byteconv.ParseFloat64(val.Bytes())
		return v, err == nil
	}
	return 0, false
}

func ToUint(val zed.Value) (uint64, bool) {
	switch id := val.Type().ID(); {
	case zed.IsUnsigned(id):
		return val.Uint(), true
	case zed.IsSigned(id):
		v := val.Int()
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case zed.IsFloat(id):
		return uint64(val.Float()), true
	case id == zed.IDString:
		v, err := byteconv.ParseUint64(val.Bytes())
		return v, err == nil
	}
	return 0, false
}

func ToInt(val zed.Value) (int64, bool) {
	switch id := val.Type().ID(); {
	case zed.IsUnsigned(id):
		return int64(val.Uint()), true
	case zed.IsSigned(id):
		// XXX check if negative? should -1:uint64 be maxint64 or an error?
		return val.Int(), true
	case zed.IsFloat(id):
		return int64(val.Float()), true
	case id == zed.IDString:
		v, err := byteconv.ParseInt64(val.Bytes())
		return v, err == nil
	}
	return 0, false
}

func ToBool(val zed.Value) (bool, bool) {
	if val.IsString() {
		v, err := byteconv.ParseBool(val.Bytes())
		return v, err == nil
	}
	v, ok := ToInt(val)
	return v != 0, ok
}

// XXX should be numToFloat and handle time, dur, eventually decimal
// (though we should promote float to decimal)
func intToFloat(val vector.Any) vector.Any {
	switch val := val.(type) {
	case *vector.Int:
		vals := val.Values
		n := int(val.Len())
		f := make([]float64, n)
		for k := 0; k < n; k++ {
			f[k] = float64(vals[k])
		}
		return vector.NewFloat(zed.TypeFloat64, f, val.Nulls)
	case *vector.Uint:
		vals := val.Values
		n := int(len(vals))
		f := make([]float64, n)
		for k := 0; k < n; k++ {
			f[k] = float64(vals[k])
		}
		return vector.NewFloat(zed.TypeFloat64, f, val.Nulls)
	case *vector.Const:
		f, ok := ToFloat(val.Value())
		if !ok {
			panic("ToFloat failed")
		}
		return vector.NewConst(nil, zed.NewFloat64(f), val.Len(), val.Nulls)
	case *vector.View:
		return vector.NewView(val.Index, intToFloat(val.Any))
	default:
		panic(fmt.Sprintf("intToFloat invalid type: %T", val))
	}
}

// XXX need intToUint (e.g, compare int to time?)
func uintToInt(val vector.Any) vector.Any {
	switch val := val.(type) {
	case *vector.Uint:
		vals := val.Values
		n := int(len(vals))
		out := make([]int64, n)
		for k := 0; k < n; k++ {
			out[k] = int64(vals[k])
		}
		return vector.NewInt(zed.TypeInt64, out, val.Nulls)
	default:
		panic(fmt.Sprintf("intToFloat invalid type: %T", val))
	}
}

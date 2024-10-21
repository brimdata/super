package expr

import (
	"math"
	"net/netip"
	"unicode/utf8"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/zson"
	"github.com/x448/float16"
)

func LookupPrimitiveCaster(zctx *super.Context, typ super.Type) Evaluator {
	switch typ {
	case super.TypeBool:
		return &casterBool{zctx}
	case super.TypeInt8:
		return &casterIntN{zctx, super.TypeInt8, math.MinInt8, math.MaxInt8}
	case super.TypeInt16:
		return &casterIntN{zctx, super.TypeInt16, math.MinInt16, math.MaxInt16}
	case super.TypeInt32:
		return &casterIntN{zctx, super.TypeInt32, math.MinInt32, math.MaxInt32}
	case super.TypeInt64:
		return &casterIntN{zctx, super.TypeInt64, 0, 0}
	case super.TypeUint8:
		return &casterUintN{zctx, super.TypeUint8, math.MaxUint8}
	case super.TypeUint16:
		return &casterUintN{zctx, super.TypeUint16, math.MaxUint16}
	case super.TypeUint32:
		return &casterUintN{zctx, super.TypeUint32, math.MaxUint32}
	case super.TypeUint64:
		return &casterUintN{zctx, super.TypeUint64, 0}
	case super.TypeFloat16:
		return &casterFloat16{zctx}
	case super.TypeFloat32:
		return &casterFloat32{zctx}
	case super.TypeFloat64:
		return &casterFloat64{zctx}
	case super.TypeIP:
		return &casterIP{zctx}
	case super.TypeNet:
		return &casterNet{zctx}
	case super.TypeDuration:
		return &casterDuration{zctx}
	case super.TypeTime:
		return &casterTime{zctx}
	case super.TypeString:
		return &casterString{zctx}
	case super.TypeBytes:
		return &casterBytes{}
	case super.TypeType:
		return &casterType{zctx}
	default:
		return nil
	}
}

type casterIntN struct {
	zctx *super.Context
	typ  super.Type
	min  int64
	max  int64
}

func (c *casterIntN) Eval(ectx Context, val super.Value) super.Value {
	v, ok := coerce.ToInt(val)
	if !ok || (c.min != 0 && (v < c.min || v > c.max)) {
		return c.zctx.WrapError("cannot cast to "+zson.FormatType(c.typ), val)
	}
	return super.NewInt(c.typ, v)
}

type casterUintN struct {
	zctx *super.Context
	typ  super.Type
	max  uint64
}

func (c *casterUintN) Eval(ectx Context, val super.Value) super.Value {
	v, ok := coerce.ToUint(val)
	if !ok || (c.max != 0 && v > c.max) {
		return c.zctx.WrapError("cannot cast to "+zson.FormatType(c.typ), val)
	}
	return super.NewUint(c.typ, v)
}

type casterBool struct {
	zctx *super.Context
}

func (c *casterBool) Eval(ectx Context, val super.Value) super.Value {
	b, ok := coerce.ToBool(val)
	if !ok {
		return c.zctx.WrapError("cannot cast to bool", val)
	}
	return super.NewBool(b)
}

type casterFloat16 struct {
	zctx *super.Context
}

func (c *casterFloat16) Eval(ectx Context, val super.Value) super.Value {
	f, ok := coerce.ToFloat(val)
	if !ok {
		return c.zctx.WrapError("cannot cast to float16", val)
	}
	f16 := float16.Fromfloat32(float32(f))
	return super.NewFloat16(f16.Float32())
}

type casterFloat32 struct {
	zctx *super.Context
}

func (c *casterFloat32) Eval(ectx Context, val super.Value) super.Value {
	f, ok := coerce.ToFloat(val)
	if !ok {
		return c.zctx.WrapError("cannot cast to float32", val)
	}
	return super.NewFloat32(float32(f))
}

type casterFloat64 struct {
	zctx *super.Context
}

func (c *casterFloat64) Eval(ectx Context, val super.Value) super.Value {
	f, ok := coerce.ToFloat(val)
	if !ok {
		return c.zctx.WrapError("cannot cast to float64", val)
	}
	return super.NewFloat64(f)
}

type casterIP struct {
	zctx *super.Context
}

func (c *casterIP) Eval(ectx Context, val super.Value) super.Value {
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfIP); ok {
		return val
	}
	if !val.IsString() {
		return c.zctx.WrapError("cannot cast to ip", val)
	}
	ip, err := byteconv.ParseIP(val.Bytes())
	if err != nil {
		return c.zctx.WrapError("cannot cast to ip", val)
	}
	return super.NewIP(ip)
}

type casterNet struct {
	zctx *super.Context
}

func (c *casterNet) Eval(ectx Context, val super.Value) super.Value {
	if val.Type().ID() == super.IDNet {
		return val
	}
	if !val.IsString() {
		return c.zctx.WrapError("cannot cast to net", val)
	}
	net, err := netip.ParsePrefix(string(val.Bytes()))
	if err != nil {
		return c.zctx.WrapError("cannot cast to net", val)
	}
	return super.NewNet(net)
}

type casterDuration struct {
	zctx *super.Context
}

func (c *casterDuration) Eval(ectx Context, val super.Value) super.Value {
	id := val.Type().ID()
	if id == super.IDDuration {
		return val
	}
	if id == super.IDString {
		d, err := nano.ParseDuration(byteconv.UnsafeString(val.Bytes()))
		if err != nil {
			f, ferr := byteconv.ParseFloat64(val.Bytes())
			if ferr != nil {
				return c.zctx.WrapError("cannot cast to duration", val)
			}
			d = nano.Duration(f)
		}
		return super.NewDuration(d)
	}
	if super.IsFloat(id) {
		return super.NewDuration(nano.Duration(val.Float()))
	}
	v, ok := coerce.ToInt(val)
	if !ok {
		return c.zctx.WrapError("cannot cast to duration", val)
	}
	return super.NewDuration(nano.Duration(v))
}

type casterTime struct {
	zctx *super.Context
}

func (c *casterTime) Eval(ectx Context, val super.Value) super.Value {
	id := val.Type().ID()
	var ts nano.Ts
	switch {
	case id == super.IDTime:
		return val
	case val.IsNull():
		// Do nothing. Any nil value is cast to a zero time.
	case id == super.IDString:
		gotime, err := dateparse.ParseAny(byteconv.UnsafeString(val.Bytes()))
		if err != nil {
			v, err := byteconv.ParseFloat64(val.Bytes())
			if err != nil {
				return c.zctx.WrapError("cannot cast to time", val)
			}
			ts = nano.Ts(v)
		} else {
			ts = nano.Ts(gotime.UnixNano())
		}
	case super.IsNumber(id):
		//XXX we call coerce on integers here to avoid unsigned/signed decode
		v, ok := coerce.ToInt(val)
		if !ok {
			return c.zctx.WrapError("cannot cast to time: coerce to int failed", val)
		}
		ts = nano.Ts(v)
	default:
		return c.zctx.WrapError("cannot cast to time", val)
	}
	return super.NewTime(ts)
}

type casterString struct {
	zctx *super.Context
}

func (c *casterString) Eval(ectx Context, val super.Value) super.Value {
	id := val.Type().ID()
	if id == super.IDBytes {
		if !utf8.Valid(val.Bytes()) {
			return c.zctx.WrapError("cannot cast to string: invalid UTF-8", val)
		}
		return super.NewValue(super.TypeString, val.Bytes())
	}
	if enum, ok := val.Type().(*super.TypeEnum); ok {
		selector := super.DecodeUint(val.Bytes())
		symbol, err := enum.Symbol(int(selector))
		if err != nil {
			return c.zctx.NewError(err)
		}
		return super.NewString(symbol)
	}
	if id == super.IDString {
		// If it's already stringy, then the Zed encoding can stay
		// the same and we just update the stringy type.
		return super.NewValue(super.TypeString, val.Bytes())
	}
	// Otherwise, we'll use a canonical ZSON value for the string rep
	// of an arbitrary value cast to a string.
	return super.NewString(zson.FormatValue(val))
}

type casterBytes struct{}

func (c *casterBytes) Eval(ectx Context, val super.Value) super.Value {
	return super.NewBytes(val.Bytes())
}

type casterNamedType struct {
	zctx *super.Context
	expr Evaluator
	name string
}

func (c *casterNamedType) Eval(ectx Context, this super.Value) super.Value {
	val := c.expr.Eval(ectx, this)
	if val.IsError() {
		return val
	}
	typ, err := c.zctx.LookupTypeNamed(c.name, super.TypeUnder(val.Type()))
	if err != nil {
		return c.zctx.NewError(err)
	}
	return super.NewValue(typ, val.Bytes())
}

type casterType struct {
	zctx *super.Context
}

func (c *casterType) Eval(ectx Context, val super.Value) super.Value {
	id := val.Type().ID()
	if id == super.IDType {
		return val
	}
	if id != super.IDString {
		return c.zctx.WrapError("cannot cast to type", val)
	}
	typval, err := zson.ParseValue(c.zctx, val.AsString())
	if err != nil {
		return c.zctx.WrapError(err.Error(), val)
	}
	if typval.Type().ID() != super.IDType {
		return c.zctx.WrapError("cannot cast to type", val)
	}
	return typval
}

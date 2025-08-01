package expr

import (
	"fmt"
	"net/netip"
	"strconv"
	"unicode/utf8"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
)

func LookupPrimitiveCaster(sctx *super.Context, typ super.Type) Evaluator {
	if enum, ok := typ.(*super.TypeEnum); ok {
		return &casterEnum{sctx, enum}
	}
	switch typ {
	case super.TypeBool:
		return &casterBool{sctx}
	case super.TypeInt8, super.TypeInt16, super.TypeInt32, super.TypeInt64:
		return &casterIntN{sctx, typ}
	case super.TypeUint8, super.TypeUint16, super.TypeUint32, super.TypeUint64:
		return &casterUintN{sctx, typ}
	case super.TypeFloat16, super.TypeFloat32, super.TypeFloat64:
		return &casterFloat{sctx, typ}
	case super.TypeIP:
		return &casterIP{sctx}
	case super.TypeNet:
		return &casterNet{sctx}
	case super.TypeDuration:
		return &casterDuration{sctx}
	case super.TypeTime:
		return &casterTime{sctx}
	case super.TypeString:
		return &casterString{sctx}
	case super.TypeBytes:
		return &casterBytes{}
	case super.TypeType:
		return &casterType{sctx}
	default:
		return nil
	}
}

type casterIntN struct {
	sctx *super.Context
	typ  super.Type
}

func (c *casterIntN) Eval(val super.Value) super.Value {
	v, ok := coerce.ToInt(val, c.typ)
	if !ok {
		return c.sctx.WrapError("cannot cast to "+sup.FormatType(c.typ), val)
	}
	return super.NewInt(c.typ, v)
}

type casterUintN struct {
	sctx *super.Context
	typ  super.Type
}

func (c *casterUintN) Eval(val super.Value) super.Value {
	v, ok := coerce.ToUint(val, c.typ)
	if !ok {
		return c.sctx.WrapError("cannot cast to "+sup.FormatType(c.typ), val)
	}
	return super.NewUint(c.typ, v)
}

type casterBool struct {
	sctx *super.Context
}

func (c *casterBool) Eval(val super.Value) super.Value {
	b, ok := coerce.ToBool(val)
	if !ok {
		return c.sctx.WrapError("cannot cast to bool", val)
	}
	return super.NewBool(b)
}

type casterFloat struct {
	sctx *super.Context
	typ  super.Type
}

func (c *casterFloat) Eval(val super.Value) super.Value {
	f, ok := coerce.ToFloat(val, c.typ)
	if !ok {
		return c.sctx.WrapError("cannot cast to "+sup.FormatType(c.typ), val)
	}
	return super.NewFloat(c.typ, f)
}

type casterIP struct {
	sctx *super.Context
}

func (c *casterIP) Eval(val super.Value) super.Value {
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfIP); ok {
		return val
	}
	if !val.IsString() {
		return c.sctx.WrapError("cannot cast to ip", val)
	}
	ip, err := byteconv.ParseIP(val.Bytes())
	if err != nil {
		return c.sctx.WrapError("cannot cast to ip", val)
	}
	return super.NewIP(ip)
}

type casterNet struct {
	sctx *super.Context
}

func (c *casterNet) Eval(val super.Value) super.Value {
	if val.Type().ID() == super.IDNet {
		return val
	}
	if !val.IsString() {
		return c.sctx.WrapError("cannot cast to net", val)
	}
	net, err := netip.ParsePrefix(string(val.Bytes()))
	if err != nil {
		return c.sctx.WrapError("cannot cast to net", val)
	}
	return super.NewNet(net)
}

type casterDuration struct {
	sctx *super.Context
}

func (c *casterDuration) Eval(val super.Value) super.Value {
	id := val.Type().ID()
	if id == super.IDDuration {
		return val
	}
	if id == super.IDString {
		d, err := nano.ParseDuration(byteconv.UnsafeString(val.Bytes()))
		if err != nil {
			f, ferr := byteconv.ParseFloat64(val.Bytes())
			if ferr != nil {
				return c.sctx.WrapError("cannot cast to duration", val)
			}
			d = nano.Duration(f)
		}
		return super.NewDuration(d)
	}
	v, ok := coerce.ToInt(val, super.TypeDuration)
	if !ok {
		return c.sctx.WrapError("cannot cast to duration", val)
	}
	return super.NewDuration(nano.Duration(v))
}

type casterTime struct {
	sctx *super.Context
}

func (c *casterTime) Eval(val super.Value) super.Value {
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
				return c.sctx.WrapError("cannot cast to time", val)
			}
			ts = nano.Ts(v)
		} else {
			ts = nano.Ts(gotime.UnixNano())
		}
	case super.IsNumber(id):
		//XXX we call coerce on integers here to avoid unsigned/signed decode
		v, ok := coerce.ToInt(val, super.TypeTime)
		if !ok {
			return c.sctx.WrapError("cannot cast to time", val)
		}
		ts = nano.Ts(v)
	default:
		return c.sctx.WrapError("cannot cast to time", val)
	}
	return super.NewTime(ts)
}

type casterString struct {
	sctx *super.Context
}

func (c *casterString) Eval(val super.Value) super.Value {
	val = val.Under()
	id := val.Type().ID()
	switch id {
	case super.IDBytes:
		if !utf8.Valid(val.Bytes()) {
			return c.sctx.WrapError("cannot cast to string: invalid UTF-8", val)
		}
		return super.NewValue(super.TypeString, val.Bytes())
	case super.IDString:
		return super.NewValue(super.TypeString, val.Bytes())
	case super.IDInt8, super.IDInt16, super.IDInt32, super.IDInt64:
		return super.NewString(strconv.FormatInt(val.Int(), 10))
	case super.IDUint8, super.IDUint16, super.IDUint32, super.IDUint64:
		return super.NewString(strconv.FormatUint(val.Uint(), 10))
	case super.IDFloat16, super.IDFloat32, super.IDFloat64:
		return super.NewString(strconv.FormatFloat(val.Float(), 'g', -1, 64))
	}
	if enum, ok := val.Type().(*super.TypeEnum); ok {
		selector := super.DecodeUint(val.Bytes())
		symbol, err := enum.Symbol(int(selector))
		if err != nil {
			return c.sctx.NewError(err)
		}
		return super.NewString(symbol)
	}
	// Otherwise, we'll use a canonical SUP value for the string rep
	// of an arbitrary value cast to a string.
	return super.NewString(sup.FormatValue(val))
}

type casterBytes struct{}

func (c *casterBytes) Eval(val super.Value) super.Value {
	return super.NewBytes(val.Bytes())
}

type casterNamedType struct {
	sctx *super.Context
	expr Evaluator
	name string
}

func (c *casterNamedType) Eval(this super.Value) super.Value {
	val := c.expr.Eval(this)
	if val.IsError() {
		return val
	}
	typ, err := c.sctx.LookupTypeNamed(c.name, super.TypeUnder(val.Type()))
	if err != nil {
		return c.sctx.NewError(err)
	}
	return super.NewValue(typ, val.Bytes())
}

type casterType struct {
	sctx *super.Context
}

func (c *casterType) Eval(val super.Value) super.Value {
	id := val.Type().ID()
	if id == super.IDType {
		return val
	}
	if id != super.IDString {
		return c.sctx.WrapError("cannot cast to type", val)
	}
	typval, err := sup.ParseValue(c.sctx, val.AsString())
	if err != nil || typval.Type().ID() != super.IDType {
		return c.sctx.WrapError("cannot cast to type", val)
	}
	return typval
}

type casterEnum struct {
	sctx *super.Context
	enum *super.TypeEnum
}

func (c *casterEnum) Eval(val super.Value) super.Value {
	id := val.Type().ID()
	if id != super.IDString {
		return c.sctx.WrapError("cannot cast to enum", val)
	}
	s := super.DecodeString(val.Bytes())
	selector := c.enum.Lookup(s)
	if selector < 0 {
		return c.sctx.WrapError(fmt.Sprintf("no such symbol in %s", sup.String(c.enum)), val)
	}
	return super.NewValue(c.enum, super.EncodeUint(uint64(selector)))
}

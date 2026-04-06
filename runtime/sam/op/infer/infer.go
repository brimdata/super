package infer

import (
	"net/netip"
	"strconv"

	"github.com/araddon/dateparse"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

//XXX we should handle numbers in general and coercible things across unions
// for example, have a inferFloat64 (for JSON/CSV numbers)
// also deal with nullables?

type infer interface {
	load(super.Type, scode.Bytes)
	typeof(*super.Context, super.Type) super.Type
}

type inferNode struct {
	children []infer
}

type inferString []string

func newInferer(typ super.Type) infer {
	switch typ := typ.(type) {
	case *super.TypeRecord:
		var fields []infer
		var n int
		for _, f := range typ.Fields {
			child := newInferer(f.Type)
			if child != nil {
				n++
			}
			fields = append(fields, child)
		}
		if n != 0 {
			return &inferNode{fields}
		}
	case *super.TypeArray:
		child := newInferer(typ.Type)
		if child == nil {
			return nil
		}
		return &inferNode{[]infer{child}}
	case *super.TypeOfString:
		return &inferString{}
	case *super.TypeUnion:
		var children []infer
		var n int
		for _, typ := range typ.Types {
			child := newInferer(typ)
			children = append(children, child)
			if child != nil {
				n++
			}
		}
		if n >= 0 {
			return &inferNode{children}
		}
	}
	return nil
}

func (i *inferString) load(typ super.Type, bytes scode.Bytes) {
	*i = append(*i, super.DecodeString(bytes))
}

func (i *inferNode) load(typ super.Type, bytes scode.Bytes) {
	switch typ := typ.(type) {
	case *super.TypeRecord:
		it := scode.NewRecordIter(bytes, typ.Opts)
		for k, f := range typ.Fields {
			elem, none := it.Next(f.Opt)
			if none {
				continue
			}
			if child := i.children[k]; child != nil {
				child.load(f.Type, elem)
			}
		}
	case *super.TypeArray:
		inner := typ.Type
		it := bytes.Iter()
		for !it.Done() {
			i.children[0].load(inner, it.Next())
		}
	case *super.TypeUnion:
		it := bytes.Iter()
		tag := super.DecodeUint(it.Next())
		inner, err := typ.Type(int(tag))
		if err != nil {
			panic(err)
		}
		if child := i.children[tag]; child != nil {
			child.load(inner, it.Next())
		}
	}
}

func (i *inferNode) typeof(sctx *super.Context, typ super.Type) super.Type {
	if i == nil {
		return typ
	}
	switch typ := typ.(type) {
	case *super.TypeRecord:
		var fields []super.Field
		for k, f := range typ.Fields {
			typ := f.Type
			if child := i.children[k]; child != nil {
				typ = child.typeof(sctx, f.Type)
			}
			fields = append(fields, super.NewFieldWithOpt(f.Name, typ, f.Opt))
		}
		return sctx.MustLookupTypeRecord(fields)
	case *super.TypeArray:
		return sctx.LookupTypeArray(i.children[0].typeof(sctx, typ.Type))
	case *super.TypeUnion:
		var types []super.Type
		for k, typ := range typ.Types {
			if child := i.children[k]; child != nil {
				typ = child.typeof(sctx, typ)
			}
			types = append(types, typ)
		}
		types = super.UniqueTypes(types)
		if len(types) == 1 {
			return types[0]
		}
		out, ok := sctx.LookupTypeUnion(types)
		if !ok {
			// infer does not create new unions so there should be
			// no way to get an anonymous union inserted into an existing union.
			panic(types)
		}
		return out
	default:
		return typ
	}
}

func (i *inferString) typeof(sctx *super.Context, typ super.Type) super.Type {
	switch {
	case i.isInt():
		return super.TypeInt64
	case i.isFloat():
		return super.TypeFloat64
	case i.isIP():
		return super.TypeIP
	case i.isNet():
		return super.TypeNet
	case i.isTime():
		return super.TypeTime
	case i.isBool():
		return super.TypeBool
	}
	return typ
}

func (i inferString) isInt() bool {
	for _, s := range i {
		if _, err := strconv.Atoi(s); err != nil {
			return false
		}
	}
	return true
}

func (i inferString) isFloat() bool {
	for _, s := range i {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return false
		}
	}
	return true
}

func (i inferString) isIP() bool {
	for _, s := range i {
		if _, err := netip.ParseAddr(s); err != nil {
			return false
		}
	}
	return true
}

func (i inferString) isNet() bool {
	for _, s := range i {
		if _, err := netip.ParsePrefix(s); err != nil {
			return false
		}
	}
	return true
}

func (i inferString) isTime() bool {
	for _, s := range i {
		if _, err := dateparse.ParseAny(s); err != nil {
			return false
		}
	}
	return true
}

func (i inferString) isBool() bool {
	for _, s := range i {
		if _, err := strconv.ParseBool(s); err != nil {
			return false
		}
	}
	return true
}

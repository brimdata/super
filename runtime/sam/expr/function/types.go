package function

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zson"
)

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	zctx *zed.Context
}

func (t *TypeOf) Call(ectx expr.Context, args []zed.Value) zed.Value {
	return t.zctx.LookupTypeValue(ectx.Arena(), args[0].Type())
}

type typeUnder struct {
	zctx *zed.Context
}

func (t *typeUnder) Call(ectx expr.Context, args []zed.Value) zed.Value {
	typ := zed.TypeUnder(args[0].Type())
	return t.zctx.LookupTypeValue(ectx.Arena(), typ)
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#nameof
type NameOf struct {
	zctx *zed.Context
}

func (n *NameOf) Call(ectx expr.Context, args []zed.Value) zed.Value {
	typ := args[0].Type()
	if named, ok := typ.(*zed.TypeNamed); ok {
		return ectx.Arena().NewString(named.Name)
	}
	if typ.ID() == zed.IDType {
		var err error
		if typ, err = n.zctx.LookupByValue(args[0].Bytes()); err != nil {
			panic(err)
		}
		if named, ok := typ.(*zed.TypeNamed); ok {
			return ectx.Arena().NewString(named.Name)
		}
	}
	return n.zctx.Missing(ectx.Arena())
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#typename
type typeName struct {
	zctx *zed.Context
}

func (t *typeName) Call(ectx expr.Context, args []zed.Value) zed.Value {
	if zed.TypeUnder(args[0].Type()) != zed.TypeString {
		return t.zctx.WrapError(ectx.Arena(), "typename: first argument not a string", args[0])
	}
	name := string(args[0].Bytes())
	if len(args) == 1 {
		typ := t.zctx.LookupTypeDef(name)
		if typ == nil {
			return t.zctx.Missing(ectx.Arena())
		}
		return t.zctx.LookupTypeValue(ectx.Arena(), typ)
	}
	if zed.TypeUnder(args[1].Type()) != zed.TypeType {
		return t.zctx.WrapError(ectx.Arena(), "typename: second argument not a type value", args[1])
	}
	typ, err := t.zctx.LookupByValue(args[1].Bytes())
	if err != nil {
		return t.zctx.NewError(ectx.Arena(), err)
	}
	return t.zctx.LookupTypeValue(ectx.Arena(), typ)
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#error
type Error struct {
	zctx *zed.Context
}

func (e *Error) Call(ectx expr.Context, args []zed.Value) zed.Value {
	return ectx.Arena().New(e.zctx.LookupTypeError(args[0].Type()), args[0].Bytes())
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#iserr
type IsErr struct{}

func (*IsErr) Call(ectx expr.Context, args []zed.Value) zed.Value {
	return zed.NewBool(args[0].IsError())
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#is
type Is struct {
	zctx *zed.Context
}

func (i *Is) Call(_ expr.Context, args []zed.Value) zed.Value {
	zvSubject := args[0]
	zvTypeVal := args[1]
	if len(args) == 3 {
		zvSubject = args[1]
		zvTypeVal = args[2]
	}
	var typ zed.Type
	var err error
	if zvTypeVal.IsString() {
		typ, err = zson.ParseType(i.zctx, string(zvTypeVal.Bytes()))
	} else {
		typ, err = i.zctx.LookupByValue(zvTypeVal.Bytes())
	}
	return zed.NewBool(err == nil && typ == zvSubject.Type())
}

type HasError struct {
	cached map[int]bool
}

func NewHasError() *HasError {
	return &HasError{
		cached: make(map[int]bool),
	}
}

func (h *HasError) Call(_ expr.Context, args []zed.Value) zed.Value {
	val := args[0]
	hasError, _ := h.hasError(val.Type(), val.Bytes())
	return zed.NewBool(hasError)
}

func (h *HasError) hasError(t zed.Type, b zcode.Bytes) (bool, bool) {
	typ := zed.TypeUnder(t)
	if _, ok := typ.(*zed.TypeError); ok {
		return true, false
	}
	// If a value is null we can skip since an null error is not an error.
	if b == nil {
		return false, false
	}
	if hasErr, ok := h.cached[t.ID()]; ok {
		return hasErr, true
	}
	var hasErr bool
	canCache := true
	switch typ := typ.(type) {
	case *zed.TypeRecord:
		it := b.Iter()
		for _, f := range typ.Fields {
			e, c := h.hasError(f.Type, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *zed.TypeArray, *zed.TypeSet:
		inner := zed.InnerType(typ)
		for it := b.Iter(); !it.Done(); {
			e, c := h.hasError(inner, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *zed.TypeMap:
		for it := b.Iter(); !it.Done(); {
			e, c := h.hasError(typ.KeyType, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
			e, c = h.hasError(typ.ValType, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *zed.TypeUnion:
		for _, typ := range typ.Types {
			_, isErr := zed.TypeUnder(typ).(*zed.TypeError)
			canCache = !canCache || isErr
		}
		if typ, b := typ.Untag(b); b != nil {
			// Check mb is not nil to avoid infinite recursion.
			var cc bool
			hasErr, cc = h.hasError(typ, b)
			canCache = !canCache || cc
		}
	}
	// We cannot cache a type if the type or one of its children has a union
	// with an error member.
	if canCache {
		h.cached[t.ID()] = hasErr
	}
	return hasErr, canCache
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#quiet
type Quiet struct {
	zctx *zed.Context
}

func (q *Quiet) Call(ectx expr.Context, args []zed.Value) zed.Value {
	val := args[0]
	if val.IsMissing() {
		return q.zctx.Quiet(ectx.Arena())
	}
	return val
}

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#kind
type Kind struct {
	zctx *zed.Context
}

func (k *Kind) Call(ectx expr.Context, args []zed.Value) zed.Value {
	val := args[0]
	var typ zed.Type
	if _, ok := zed.TypeUnder(val.Type()).(*zed.TypeOfType); ok {
		var err error
		typ, err = k.zctx.LookupByValue(val.Bytes())
		if err != nil {
			panic(err)
		}
	} else {
		typ = val.Type()
	}
	return ectx.Arena().NewString(typ.Kind().String())
}

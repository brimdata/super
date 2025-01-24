package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	zctx *super.Context
}

func (t *TypeOf) Call(_ super.Allocator, args []super.Value) super.Value {
	return t.zctx.LookupTypeValue(args[0].Type())
}

type typeUnder struct {
	zctx *super.Context
}

func (t *typeUnder) Call(_ super.Allocator, args []super.Value) super.Value {
	typ := super.TypeUnder(args[0].Type())
	return t.zctx.LookupTypeValue(typ)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#nameof
type NameOf struct {
	zctx *super.Context
}

func (n *NameOf) Call(_ super.Allocator, args []super.Value) super.Value {
	typ := args[0].Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return super.NewString(named.Name)
	}
	if typ.ID() == super.IDType {
		var err error
		if typ, err = n.zctx.LookupByValue(args[0].Bytes()); err != nil {
			panic(err)
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			return super.NewString(named.Name)
		}
	}
	return n.zctx.Missing()
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typename
type typeName struct {
	zctx *super.Context
}

func (t *typeName) Call(_ super.Allocator, args []super.Value) super.Value {
	if super.TypeUnder(args[0].Type()) != super.TypeString {
		return t.zctx.WrapError("typename: argument must be a string", args[0])
	}
	name := string(args[0].Bytes())
	typ := t.zctx.LookupTypeDef(name)
	if typ == nil {
		return t.zctx.Missing()
	}
	return t.zctx.LookupTypeValue(typ)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#error
type Error struct {
	zctx *super.Context
}

func (e *Error) Call(_ super.Allocator, args []super.Value) super.Value {
	return super.NewValue(e.zctx.LookupTypeError(args[0].Type()), args[0].Bytes())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#iserr
type IsErr struct{}

func (*IsErr) Call(_ super.Allocator, args []super.Value) super.Value {
	return super.NewBool(args[0].IsError())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#is
type Is struct {
	zctx *super.Context
}

func (i *Is) Call(_ super.Allocator, args []super.Value) super.Value {
	zvSubject := args[0]
	zvTypeVal := args[1]
	if len(args) == 3 {
		zvSubject = args[1]
		zvTypeVal = args[2]
	}
	var typ super.Type
	var err error
	if zvTypeVal.IsString() {
		typ, err = zson.ParseType(i.zctx, string(zvTypeVal.Bytes()))
	} else {
		typ, err = i.zctx.LookupByValue(zvTypeVal.Bytes())
	}
	return super.NewBool(err == nil && typ == zvSubject.Type())
}

type HasError struct {
	cached map[int]bool
}

func NewHasError() *HasError {
	return &HasError{
		cached: make(map[int]bool),
	}
}

func (h *HasError) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	hasError, _ := h.hasError(val.Type(), val.Bytes())
	return super.NewBool(hasError)
}

func (h *HasError) hasError(t super.Type, b zcode.Bytes) (bool, bool) {
	typ := super.TypeUnder(t)
	if _, ok := typ.(*super.TypeError); ok {
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
	case *super.TypeRecord:
		it := b.Iter()
		for _, f := range typ.Fields {
			e, c := h.hasError(f.Type, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *super.TypeArray, *super.TypeSet:
		inner := super.InnerType(typ)
		for it := b.Iter(); !it.Done(); {
			e, c := h.hasError(inner, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *super.TypeMap:
		for it := b.Iter(); !it.Done(); {
			e, c := h.hasError(typ.KeyType, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
			e, c = h.hasError(typ.ValType, it.Next())
			hasErr = hasErr || e
			canCache = !canCache || c
		}
	case *super.TypeUnion:
		for _, typ := range typ.Types {
			_, isErr := super.TypeUnder(typ).(*super.TypeError)
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

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#quiet
type Quiet struct {
	zctx *super.Context
}

func (q *Quiet) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	if val.IsMissing() {
		return q.zctx.Quiet()
	}
	return val
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#kind
type Kind struct {
	zctx *super.Context
}

func (k *Kind) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	var typ super.Type
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfType); ok {
		var err error
		typ, err = k.zctx.LookupByValue(val.Bytes())
		if err != nil {
			panic(err)
		}
	} else {
		typ = val.Type()
	}
	return super.NewString(typ.Kind().String())
}

package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	sctx *super.Context
}

func (t *TypeOf) Call(_ super.Allocator, args []super.Value) super.Value {
	return t.sctx.LookupTypeValue(args[0].Type())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#nameof
type NameOf struct {
	sctx *super.Context
}

func (n *NameOf) Call(_ super.Allocator, args []super.Value) super.Value {
	typ := args[0].Type()
	if named, ok := typ.(*super.TypeNamed); ok {
		return super.NewString(named.Name)
	}
	if typ.ID() == super.IDType {
		if args[0].IsNull() {
			return super.NullString
		}
		var err error
		if typ, err = n.sctx.LookupByValue(args[0].Bytes()); err != nil {
			panic(err)
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			return super.NewString(named.Name)
		}
	}
	return n.sctx.Missing()
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typename
type typeName struct {
	sctx *super.Context
}

func (t *typeName) Call(_ super.Allocator, args []super.Value) super.Value {
	if super.TypeUnder(args[0].Type()) != super.TypeString {
		return t.sctx.WrapError("typename: argument must be a string", args[0])
	}
	name := string(args[0].Bytes())
	typ := t.sctx.LookupTypeDef(name)
	if typ == nil {
		return t.sctx.Missing()
	}
	return t.sctx.LookupTypeValue(typ)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#error
type Error struct {
	sctx *super.Context
}

func (e *Error) Call(_ super.Allocator, args []super.Value) super.Value {
	return super.NewValue(e.sctx.LookupTypeError(args[0].Type()), args[0].Bytes())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#iserr
type IsErr struct{}

func (*IsErr) Call(_ super.Allocator, args []super.Value) super.Value {
	return super.NewBool(args[0].IsError())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#is
type Is struct {
	sctx *super.Context
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
		typ, err = sup.ParseType(i.sctx, string(zvTypeVal.Bytes()))
	} else {
		typ, err = i.sctx.LookupByValue(zvTypeVal.Bytes())
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
	sctx *super.Context
}

func (q *Quiet) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	if val.IsMissing() {
		return q.sctx.Quiet()
	}
	return val
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#kind
type Kind struct {
	sctx *super.Context
}

func (k *Kind) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	var typ super.Type
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfType); ok {
		var err error
		typ, err = k.sctx.LookupByValue(val.Bytes())
		if err != nil {
			panic(err)
		}
	} else {
		typ = val.Type()
	}
	return super.NewString(typ.Kind().String())
}

package function

import (
	"strings"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#replace
type Replace struct {
	zctx *super.Context
}

func (r *Replace) Call(_ super.Allocator, args []super.Value) super.Value {
	args = underAll(args)
	sVal := args[0]
	oldVal := args[1]
	newVal := args[2]
	for i := range args {
		if !args[i].IsString() {
			return r.zctx.WrapError("replace: string arg required", args[i])
		}
	}
	if sVal.IsNull() {
		return super.Null
	}
	if oldVal.IsNull() || newVal.IsNull() {
		return r.zctx.NewErrorf("replace: an input arg is null")
	}
	s := super.DecodeString(sVal.Bytes())
	old := super.DecodeString(oldVal.Bytes())
	new := super.DecodeString(newVal.Bytes())
	return super.NewString(strings.ReplaceAll(s, old, new))
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#run_len
type RuneLen struct {
	zctx *super.Context
}

func (r *RuneLen) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return r.zctx.WrapError("rune_len: string arg required", val)
	}
	if val.IsNull() {
		return super.NewInt64(0)
	}
	s := super.DecodeString(val.Bytes())
	return super.NewInt64(int64(utf8.RuneCountInString(s)))
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#lower
type ToLower struct {
	zctx *super.Context
}

func (t *ToLower) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.zctx.WrapError("lower: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.ToLower(s))
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#upper
type ToUpper struct {
	zctx *super.Context
}

func (t *ToUpper) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.zctx.WrapError("upper: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.ToUpper(s))
}

type Trim struct {
	zctx *super.Context
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#trim
func (t *Trim) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.zctx.WrapError("trim: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.TrimSpace(s))
}

// // https://github.com/brimdata/super/blob/main/docs/language/functions.md#split
type Split struct {
	zctx *super.Context
	typ  super.Type
}

func newSplit(zctx *super.Context) *Split {
	return &Split{
		zctx: zctx,
		typ:  zctx.LookupTypeArray(super.TypeString),
	}
}

func (s *Split) Call(_ super.Allocator, args []super.Value) super.Value {
	args = underAll(args)
	for i := range args {
		if !args[i].IsString() {
			return s.zctx.WrapError("split: string arg required", args[i])
		}
	}
	sVal, sepVal := args[0], args[1]
	if sVal.IsNull() || sepVal.IsNull() {
		return super.NewValue(s.typ, nil)
	}
	str := super.DecodeString(sVal.Bytes())
	sep := super.DecodeString(sepVal.Bytes())
	splits := strings.Split(str, sep)
	var b zcode.Bytes
	for _, substr := range splits {
		b = zcode.Append(b, super.EncodeString(substr))
	}
	return super.NewValue(s.typ, b)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#join
type Join struct {
	zctx    *super.Context
	builder strings.Builder
}

func (j *Join) Call(_ super.Allocator, args []super.Value) super.Value {
	args = underAll(args)
	splitsVal := args[0]
	typ, ok := super.TypeUnder(splitsVal.Type()).(*super.TypeArray)
	if !ok || typ.Type.ID() != super.IDString {
		return j.zctx.WrapError("join: array of string arg required", splitsVal)
	}
	var separator string
	if len(args) == 2 {
		sepVal := args[1]
		if !sepVal.IsString() {
			return j.zctx.WrapError("join: separator must be string", sepVal)
		}
		separator = super.DecodeString(sepVal.Bytes())
	}
	b := j.builder
	b.Reset()
	it := splitsVal.Bytes().Iter()
	var sep string
	for !it.Done() {
		b.WriteString(sep)
		b.WriteString(super.DecodeString(it.Next()))
		sep = separator
	}
	return super.NewString(b.String())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#levenshtein
type Levenshtein struct {
	zctx *super.Context
}

func (l *Levenshtein) Call(_ super.Allocator, args []super.Value) super.Value {
	args = underAll(args)
	a, b := args[0], args[1]
	if !a.IsString() {
		return l.zctx.WrapError("levenshtein: string args required", a)
	}
	if !b.IsString() {
		return l.zctx.WrapError("levenshtein: string args required", b)
	}
	as, bs := super.DecodeString(a.Bytes()), super.DecodeString(b.Bytes())
	return super.NewInt64(int64(levenshtein.ComputeDistance(as, bs)))
}

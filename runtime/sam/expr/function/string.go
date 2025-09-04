package function

import (
	"strings"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Position struct {
	sctx *super.Context
}

func (p *Position) Call(args []super.Value) super.Value {
	val, subVal := args[0], args[1]
	if !val.IsString() {
		return p.sctx.WrapError("position: string arguments required", val)
	}
	if !subVal.IsString() {
		return p.sctx.WrapError("position: string arguments required", subVal)
	}
	if val.IsNull() || subVal.IsNull() {
		return super.NullInt64
	}
	i := strings.Index(val.AsString(), subVal.AsString())
	return super.NewInt64(int64(i + 1))
}

type Replace struct {
	sctx *super.Context
}

func (r *Replace) Call(args []super.Value) super.Value {
	args = underAll(args)
	sVal := args[0]
	oldVal := args[1]
	newVal := args[2]
	for i := range args {
		if !args[i].IsString() {
			return r.sctx.WrapError("replace: string arg required", args[i])
		}
	}
	if sVal.IsNull() {
		return super.Null
	}
	if oldVal.IsNull() || newVal.IsNull() {
		return r.sctx.NewErrorf("replace: an input arg is null")
	}
	s := super.DecodeString(sVal.Bytes())
	old := super.DecodeString(oldVal.Bytes())
	new := super.DecodeString(newVal.Bytes())
	return super.NewString(strings.ReplaceAll(s, old, new))
}

type RuneLen struct {
	sctx *super.Context
}

func (r *RuneLen) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return r.sctx.WrapError("rune_len: string arg required", val)
	}
	if val.IsNull() {
		return super.NewInt64(0)
	}
	s := super.DecodeString(val.Bytes())
	return super.NewInt64(int64(utf8.RuneCountInString(s)))
}

type ToLower struct {
	sctx *super.Context
}

func (t *ToLower) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.sctx.WrapError("lower: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.ToLower(s))
}

type ToUpper struct {
	sctx *super.Context
}

func (t *ToUpper) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.sctx.WrapError("upper: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.ToUpper(s))
}

type Trim struct {
	sctx *super.Context
}

func (t *Trim) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if !val.IsString() {
		return t.sctx.WrapError("trim: string arg required", val)
	}
	if val.IsNull() {
		return super.NullString
	}
	s := super.DecodeString(val.Bytes())
	return super.NewString(strings.TrimSpace(s))
}

type Split struct {
	sctx *super.Context
	typ  super.Type
}

func newSplit(sctx *super.Context) *Split {
	return &Split{
		sctx: sctx,
		typ:  sctx.LookupTypeArray(super.TypeString),
	}
}

func (s *Split) Call(args []super.Value) super.Value {
	args = underAll(args)
	for i := range args {
		if !args[i].IsString() {
			return s.sctx.WrapError("split: string arg required", args[i])
		}
	}
	sVal, sepVal := args[0], args[1]
	if sVal.IsNull() || sepVal.IsNull() {
		return super.NewValue(s.typ, nil)
	}
	str := super.DecodeString(sVal.Bytes())
	sep := super.DecodeString(sepVal.Bytes())
	splits := strings.Split(str, sep)
	var b scode.Bytes
	for _, substr := range splits {
		b = scode.Append(b, super.EncodeString(substr))
	}
	return super.NewValue(s.typ, b)
}

type Join struct {
	sctx    *super.Context
	builder strings.Builder
}

func (j *Join) Call(args []super.Value) super.Value {
	args = underAll(args)
	splitsVal := args[0]
	typ, ok := super.TypeUnder(splitsVal.Type()).(*super.TypeArray)
	if !ok || typ.Type.ID() != super.IDString {
		return j.sctx.WrapError("join: array of string arg required", splitsVal)
	}
	var separator string
	if len(args) == 2 {
		sepVal := args[1]
		if !sepVal.IsString() {
			return j.sctx.WrapError("join: separator must be string", sepVal)
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

type Levenshtein struct {
	sctx *super.Context
}

func (l *Levenshtein) Call(args []super.Value) super.Value {
	args = underAll(args)
	a, b := args[0], args[1]
	if !a.IsString() {
		return l.sctx.WrapError("levenshtein: string args required", a)
	}
	if !b.IsString() {
		return l.sctx.WrapError("levenshtein: string args required", b)
	}
	as, bs := super.DecodeString(a.Bytes()), super.DecodeString(b.Bytes())
	return super.NewInt64(int64(levenshtein.ComputeDistance(as, bs)))
}

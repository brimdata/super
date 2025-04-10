package function

import (
	"strings"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

// // https://github.com/brimdata/super/blob/main/docs/language/functions.md#join
type Join struct {
	sctx    *super.Context
	builder strings.Builder
}

func (j *Join) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	splitsVal := args[0]
	typ, ok := splitsVal.Type().(*super.TypeArray)
	if !ok || typ.Type.ID() != super.IDString {
		return vector.NewWrappedError(j.sctx, "join: array of string arg required", splitsVal)
	}
	var sepVal vector.Any
	if len(args) == 2 {
		if sepVal = args[1]; sepVal.Type() != super.TypeString {
			return vector.NewWrappedError(j.sctx, "join: separator must be string", sepVal)
		}
	}
	out := vector.NewStringEmpty(0, bitvec.NewFalse(splitsVal.Len()))
	inner := vector.Inner(splitsVal)
	for i := uint32(0); i < splitsVal.Len(); i++ {
		var seperator string
		if sepVal != nil {
			seperator, _ = vector.StringValue(sepVal, i)
		}
		off, end, null := vector.ContainerOffset(splitsVal, i)
		if null {
			out.Nulls.Set(i)
		}
		j.builder.Reset()
		var sep string
		for ; off < end; off++ {
			s, _ := vector.StringValue(inner, off)
			j.builder.WriteString(sep)
			j.builder.WriteString(s)
			sep = seperator
		}
		out.Append(j.builder.String())
	}
	return out
}

// // https://github.com/brimdata/super/blob/main/docs/language/functions.md#levenshtein
type Levenshtein struct {
	sctx *super.Context
}

func (l *Levenshtein) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for _, a := range args {
		if a.Type() != super.TypeString {
			return vector.NewWrappedError(l.sctx, "levenshtein: string args required", a)
		}
	}
	a, b := args[0], args[1]
	out := vector.NewIntEmpty(super.TypeInt64, a.Len(), bitvec.Zero)
	for i := uint32(0); i < a.Len(); i++ {
		as, _ := vector.StringValue(a, i)
		bs, _ := vector.StringValue(b, i)
		out.Append(int64(levenshtein.ComputeDistance(as, bs)))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#replace
type Replace struct {
	sctx *super.Context
}

func (r *Replace) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for _, arg := range args {
		if arg.Type() != super.TypeString {
			return vector.NewWrappedError(r.sctx, "replace: string arg required", arg)
		}
	}
	var errcnt uint32
	sVal := args[0]
	tags := make([]uint32, sVal.Len())
	out := vector.NewStringEmpty(0, bitvec.NewFalse(sVal.Len()))
	for i := uint32(0); i < sVal.Len(); i++ {
		s, snull := vector.StringValue(sVal, i)
		old, oldnull := vector.StringValue(args[1], i)
		new, newnull := vector.StringValue(args[2], i)
		if oldnull || newnull {
			tags[i] = 1
			errcnt++
			continue
		}
		if snull {
			out.Nulls.Set(out.Len())
		}
		out.Append(strings.ReplaceAll(s, old, new))
	}
	errval := vector.NewStringError(r.sctx, "replace: an input arg is null", errcnt)
	return vector.NewDynamic(tags, []vector.Any{out, errval})
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#run_len
type RuneLen struct {
	sctx *super.Context
}

func (r *RuneLen) Call(args ...vector.Any) vector.Any {
	val := underAll(args)[0]
	if val.Type() != super.TypeString {
		return vector.NewWrappedError(r.sctx, "rune_len: string arg required", val)
	}
	out := vector.NewIntEmpty(super.TypeInt64, val.Len(), bitvec.NewFalse(val.Len()))
	for i := uint32(0); i < val.Len(); i++ {
		s, null := vector.StringValue(val, i)
		if null {
			out.Nulls.Set(i)
		}
		out.Append(int64(utf8.RuneCountInString(s)))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#split
type Split struct {
	sctx *super.Context
}

func (s *Split) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for i := range args {
		if args[i].Type() != super.TypeString {
			return vector.NewWrappedError(s.sctx, "split: string arg required", args[i])
		}
	}
	sVal, sepVal := args[0], args[1]
	var offsets []uint32
	values := vector.NewStringEmpty(0, bitvec.Zero)
	nulls := bitvec.NewFalse(sVal.Len())
	var off uint32
	for i := uint32(0); i < sVal.Len(); i++ {
		ss, snull := vector.StringValue(sVal, i)
		sep, sepnull := vector.StringValue(sepVal, i)
		if snull || sepnull {
			offsets = append(offsets, off)
			nulls.Set(i)
			continue
		}
		splits := strings.Split(ss, sep)
		for _, substr := range splits {
			values.Append(substr)
		}
		offsets = append(offsets, off)
		off += uint32(len(splits))
	}
	offsets = append(offsets, off)
	return vector.NewArray(s.sctx.LookupTypeArray(super.TypeString), offsets, values, nulls)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#lower
type ToLower struct {
	sctx *super.Context
}

func (t *ToLower) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "lower: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len(), bitvec.NewFalse(v.Len()))
	for i := uint32(0); i < v.Len(); i++ {
		s, null := vector.StringValue(v, i)
		if null {
			out.Nulls.Set(i)
		}
		out.Append(strings.ToLower(s))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#upper
type ToUpper struct {
	sctx *super.Context
}

func (t *ToUpper) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "upper: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len(), bitvec.NewFalse(v.Len()))
	for i := uint32(0); i < v.Len(); i++ {
		s, null := vector.StringValue(v, i)
		if null {
			out.Nulls.Set(i)
		}
		out.Append(strings.ToUpper(s))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#trim
type Trim struct {
	sctx *super.Context
}

func (t *Trim) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	if val.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "trim: string arg required", val)
	}
	out := vector.NewStringEmpty(val.Len(), bitvec.NewFalse(val.Len()))
	for i := uint32(0); i < val.Len(); i++ {
		s, null := vector.StringValue(val, i)
		if null {
			out.Nulls.Set(i)
		}
		out.Append(strings.TrimSpace(s))
	}
	return out
}

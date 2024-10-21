package function

import (
	"strings"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// // https://github.com/brimdata/super/blob/main/docs/language/functions.md#join
type Join struct {
	zctx    *super.Context
	builder strings.Builder
}

func (j *Join) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	splitsVal := args[0]
	typ, ok := splitsVal.Type().(*super.TypeArray)
	if !ok || typ.Type.ID() != super.IDString {
		return vector.NewWrappedError(j.zctx, "join: array of string arg required", splitsVal)
	}
	var sepVal vector.Any
	if len(args) == 2 {
		if sepVal = args[1]; sepVal.Type() != super.TypeString {
			return vector.NewWrappedError(j.zctx, "join: separator must be string", sepVal)
		}
	}
	out := vector.NewStringEmpty(0, vector.NewBoolEmpty(splitsVal.Len(), nil))
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
	zctx *super.Context
}

func (l *Levenshtein) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for _, a := range args {
		if a.Type() != super.TypeString {
			return vector.NewWrappedError(l.zctx, "levenshtein: string args required", a)
		}
	}
	a, b := args[0], args[1]
	out := vector.NewIntEmpty(super.TypeInt64, a.Len(), nil)
	for i := uint32(0); i < a.Len(); i++ {
		as, _ := vector.StringValue(a, i)
		bs, _ := vector.StringValue(b, i)
		out.Append(int64(levenshtein.ComputeDistance(as, bs)))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#replace
type Replace struct {
	zctx *super.Context
}

func (r *Replace) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for _, arg := range args {
		if arg.Type() != super.TypeString {
			return vector.NewWrappedError(r.zctx, "replace: string arg required", arg)
		}
	}
	var errcnt uint32
	sVal := args[0]
	tags := make([]uint32, sVal.Len())
	out := vector.NewStringEmpty(0, vector.NewBoolEmpty(sVal.Len(), nil))
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
	errval := vector.NewStringError(r.zctx, "replace: an input arg is null", errcnt)
	return vector.NewDynamic(tags, []vector.Any{out, errval})
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#run_len
type RuneLen struct {
	zctx *super.Context
}

func (r *RuneLen) Call(args ...vector.Any) vector.Any {
	val := underAll(args)[0]
	if val.Type() != super.TypeString {
		return vector.NewWrappedError(r.zctx, "rune_len: string arg required", val)
	}
	out := vector.NewIntEmpty(super.TypeInt64, val.Len(), vector.NewBoolEmpty(val.Len(), nil))
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
	zctx *super.Context
}

func (s *Split) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for i := range args {
		if args[i].Type() != super.TypeString {
			return vector.NewWrappedError(s.zctx, "split: string arg required", args[i])
		}
	}
	sVal, sepVal := args[0], args[1]
	var offsets []uint32
	values := vector.NewStringEmpty(0, nil)
	nulls := vector.NewBoolEmpty(sVal.Len(), nil)
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
	return vector.NewArray(s.zctx.LookupTypeArray(super.TypeString), offsets, values, nulls)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#lower
type ToLower struct {
	zctx *super.Context
}

func (t *ToLower) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.zctx, "lower: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len(), vector.NewBoolEmpty(v.Len(), nil))
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
	zctx *super.Context
}

func (t *ToUpper) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.zctx, "upper: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len(), vector.NewBoolEmpty(v.Len(), nil))
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
	zctx *super.Context
}

func (t *Trim) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	if val.Type() != super.TypeString {
		return vector.NewWrappedError(t.zctx, "trim: string arg required", val)
	}
	out := vector.NewStringEmpty(val.Len(), vector.NewBoolEmpty(val.Len(), nil))
	for i := uint32(0); i < val.Len(); i++ {
		s, null := vector.StringValue(val, i)
		if null {
			out.Nulls.Set(i)
		}
		out.Append(strings.TrimSpace(s))
	}
	return out
}

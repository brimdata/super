package function

import (
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Concat struct {
	sctx    *super.Context
	builder strings.Builder
}

func (c *Concat) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	for _, arg := range args {
		switch arg.Kind() {
		case vector.KindError:
			return arg
		case vector.KindString:
		default:
			return vector.NewWrappedError(c.sctx, "concat: string arg required", arg)
		}
	}
	n := args[0].Len()
	out := vector.NewStringEmpty(0)
	for i := range n {
		c.builder.Reset()
		for _, arg := range args {
			s := vector.StringValue(arg, i)
			c.builder.WriteString(s)
		}
		out.Append(c.builder.String())
	}
	return out
}

type Join struct {
	sctx    *super.Context
	builder strings.Builder
}

func (j *Join) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
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
	out := vector.NewStringEmpty(0)
	inner := vector.Inner(splitsVal)
	for i := uint32(0); i < splitsVal.Len(); i++ {
		var seperator string
		if sepVal != nil {
			seperator = vector.StringValue(sepVal, i)
		}
		off, end := vector.ContainerOffset(splitsVal, i)
		j.builder.Reset()
		var sep string
		for ; off < end; off++ {
			s := vector.StringValue(inner, off)
			j.builder.WriteString(sep)
			j.builder.WriteString(s)
			sep = seperator
		}
		out.Append(j.builder.String())
	}
	return out
}

type Levenshtein struct {
	sctx *super.Context
}

func (l *Levenshtein) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	for _, a := range args {
		if a.Type() != super.TypeString {
			return vector.NewWrappedError(l.sctx, "levenshtein: string args required", a)
		}
	}
	a, b := args[0], args[1]
	out := vector.NewIntEmpty(super.TypeInt64, a.Len())
	for i := uint32(0); i < a.Len(); i++ {
		as := vector.StringValue(a, i)
		bs := vector.StringValue(b, i)
		out.Append(int64(levenshtein.ComputeDistance(as, bs)))
	}
	return out
}

type Position struct {
	sctx *super.Context
}

func (p *Position) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	vec, subVec := args[0], args[1]
	if vec.Type().ID() != super.IDString {
		return vector.NewWrappedError(p.sctx, "position: string arguments required", vec)
	}
	if subVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(p.sctx, "position: string arguments required", subVec)
	}
	vals := make([]int64, vec.Len())
	for i := range vec.Len() {
		s := vector.StringValue(vec, i)
		sub := vector.StringValue(subVec, i)
		vals[i] = int64(strings.Index(s, sub) + 1)
	}
	return vector.NewInt(super.TypeInt64, vals)
}

type Replace struct {
	sctx *super.Context
}

func (r *Replace) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	for _, arg := range args {
		if arg.Type() != super.TypeString {
			return vector.NewWrappedError(r.sctx, "replace: string arg required", arg)
		}
	}
	sVal := args[0]
	out := vector.NewStringEmpty(0)
	for i := range sVal.Len() {
		s := vector.StringValue(sVal, i)
		old := vector.StringValue(args[1], i)
		new := vector.StringValue(args[2], i)
		out.Append(strings.ReplaceAll(s, old, new))
	}
	return out
}

type Split struct {
	sctx *super.Context
}

func (s *Split) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	for i := range args {
		if args[i].Type() != super.TypeString {
			return vector.NewWrappedError(s.sctx, "split: string arg required", args[i])
		}
	}
	sVal, sepVal := args[0], args[1]
	var offsets []uint32
	values := vector.NewStringEmpty(0)
	var off uint32
	for i := uint32(0); i < sVal.Len(); i++ {
		ss := vector.StringValue(sVal, i)
		sep := vector.StringValue(sepVal, i)
		splits := strings.Split(ss, sep)
		for _, substr := range splits {
			values.Append(substr)
		}
		offsets = append(offsets, off)
		off += uint32(len(splits))
	}
	offsets = append(offsets, off)
	return vector.NewArray(s.sctx.LookupTypeArray(super.TypeString), offsets, values)
}

type ToLower struct {
	sctx *super.Context
}

func (t *ToLower) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Kind() == vector.KindNull {
		return v
	}
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "lower: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len())
	for i := uint32(0); i < v.Len(); i++ {
		s := vector.StringValue(v, i)
		out.Append(strings.ToLower(s))
	}
	return out
}

type ToUpper struct {
	sctx *super.Context
}

func (t *ToUpper) Call(args ...vector.Any) vector.Any {
	v := vector.Under(args[0])
	if v.Kind() == vector.KindNull {
		return v
	}
	if v.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "upper: string arg required", v)
	}
	out := vector.NewStringEmpty(v.Len())
	for i := uint32(0); i < v.Len(); i++ {
		s := vector.StringValue(v, i)
		out.Append(strings.ToUpper(s))
	}
	return out
}

type Trim struct {
	sctx *super.Context
}

func (t *Trim) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	if val.Kind() == vector.KindNull {
		return val
	}
	if val.Type() != super.TypeString {
		return vector.NewWrappedError(t.sctx, "trim: string arg required", val)
	}
	out := vector.NewStringEmpty(val.Len())
	for i := uint32(0); i < val.Len(); i++ {
		s := vector.StringValue(val, i)
		out.Append(strings.TrimSpace(s))
	}
	return out
}

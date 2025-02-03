package function

import (
	"regexp"
	"regexp/syntax"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#regexp
type Regexp struct {
	builder zcode.Builder
	re      *regexp.Regexp
	restr   string
	typ     super.Type
	err     error
	zctx    *super.Context
}

func (r *Regexp) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	regVec, inputVec := args[0], args[1]
	if regVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(r.zctx, "regexp: string required for first arg", args[0])
	}
	if inputVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(r.zctx, "regexp: string required for second arg", args[1])
	}
	errMsg := vector.NewStringEmpty(0, nil)
	var errs []uint32
	inner := vector.NewStringEmpty(0, nil)
	out := vector.NewArray(r.zctx.LookupTypeArray(super.TypeString), []uint32{0}, inner, nil)
	for i := range regVec.Len() {
		re, _ := vector.StringValue(regVec, i)
		if r.restr != re {
			r.restr = re
			r.re, r.err = regexp.Compile(r.restr)
		}
		if r.err != nil {
			msg := "regexp: invalid regular expression"
			if syntaxErr, ok := r.err.(*syntax.Error); ok {
				msg += ": " + syntaxErr.Code.String()
			}
			errMsg.Append(msg)
			errs = append(errs, i)
			continue
		}
		s, _ := vector.StringValue(inputVec, i)
		match := r.re.FindStringSubmatch(s)
		if match == nil {
			if out.Nulls == nil {
				out.Nulls = vector.NewBoolEmpty(regVec.Len(), nil)
			}
			out.Nulls.Set(out.Len())
			out.Offsets = append(out.Offsets, inner.Len())
			continue
		}
		for _, b := range match {
			inner.Append(b)
		}
		out.Offsets = append(out.Offsets, inner.Len())
	}
	out.Nulls.SetLen(out.Len())
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewVecWrappedError(r.zctx, errMsg, vector.NewView(regVec, errs)))
	}
	return out
}

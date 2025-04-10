package function

import (
	"fmt"
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
	sctx    *super.Context
}

func (r *Regexp) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	regVec, inputVec := args[0], args[1]
	if regVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(r.sctx, "regexp: string required for first arg", args[0])
	}
	if inputVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(r.sctx, "regexp: string required for second arg", args[1])
	}
	errMsg := vector.NewStringEmpty(0, nil)
	var errs []uint32
	inner := vector.NewStringEmpty(0, nil)
	out := vector.NewArray(r.sctx.LookupTypeArray(super.TypeString), []uint32{0}, inner, nil)
	for i := range regVec.Len() {
		re, _ := vector.StringValue(regVec, i)
		if r.restr != re {
			r.restr = re
			r.re, r.err = regexp.Compile(r.restr)
		}
		if r.err != nil {
			errMsg.Append(regexpErrMsg("regexp", r.err))
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
		return vector.Combine(out, errs, vector.NewVecWrappedError(r.sctx, errMsg, vector.Pick(regVec, errs)))
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#regexp_replace
type RegexpReplace struct {
	sctx  *super.Context
	re    *regexp.Regexp
	restr string
	err   error
}

func (r *RegexpReplace) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	for i := range args {
		if args[i].Type().ID() != super.IDString {
			return vector.NewWrappedError(r.sctx, "regexp_replace: string arg required", args[i])
		}
	}
	sVec := args[0]
	reVec := args[1]
	replaceVec := args[2]
	errMsg := vector.NewStringEmpty(0, nil)
	var errs []uint32
	nulls := vector.Or(vector.Or(vector.NullsOf(sVec), vector.NullsOf(reVec)), vector.NullsOf(replaceVec))
	out := vector.NewStringEmpty(0, nulls)
	for i := range sVec.Len() {
		s, null := vector.StringValue(sVec, i)
		if null {
			out.Append("")
			continue
		}
		re, null := vector.StringValue(reVec, i)
		if null {
			out.Append("")
			continue
		}
		replace, null := vector.StringValue(replaceVec, i)
		if null {
			out.Append("")
			continue
		}
		if r.restr != re {
			r.restr = re
			r.re, r.err = regexp.Compile(re)
		}
		if r.err != nil {
			errMsg.Append(regexpErrMsg("regexp_replace", r.err))
			errs = append(errs, i)
			continue
		}
		out.Append(r.re.ReplaceAllString(s, replace))
	}
	if len(errs) > 0 {
		out.Nulls = vector.ReversePick(out.Nulls, errs).(*vector.Bool)
		return vector.Combine(out, errs, vector.NewVecWrappedError(r.sctx, errMsg, vector.Pick(args[1], errs)))
	}
	return out
}

func regexpErrMsg(fn string, err error) string {
	msg := fmt.Sprintf("%s: invalid regular expression", fn)
	if syntaxErr, ok := err.(*syntax.Error); ok {
		msg += ": " + syntaxErr.Code.String()
	}
	return msg
}

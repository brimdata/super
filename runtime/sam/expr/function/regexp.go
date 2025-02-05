package function

import (
	"regexp"
	"regexp/syntax"

	"github.com/brimdata/super"
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

func (r *Regexp) Call(_ super.Allocator, args []super.Value) super.Value {
	if !args[0].IsString() {
		return r.zctx.WrapError("regexp: string required for first arg", args[0])
	}
	s := super.DecodeString(args[0].Bytes())
	if r.restr != s {
		r.restr = s
		r.re, r.err = regexp.Compile(r.restr)
	}
	if r.err != nil {
		msg := "regexp: invalid regular expression"
		if syntaxErr, ok := r.err.(*syntax.Error); ok {
			msg += ": " + syntaxErr.Code.String()
		}
		return r.zctx.WrapError(msg, args[0])
	}
	if !args[1].IsString() {
		return r.zctx.WrapError("regexp: string required for second arg", args[1])
	}
	r.builder.Reset()
	for _, b := range r.re.FindSubmatch(args[1].Bytes()) {
		r.builder.Append(b)
	}
	if r.typ == nil {
		r.typ = r.zctx.LookupTypeArray(super.TypeString)
	}
	return super.NewValue(r.typ, r.builder.Bytes())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#regexp_replace
type RegexpReplace struct {
	zctx  *super.Context
	re    *regexp.Regexp
	restr string
	err   error
}

func (r *RegexpReplace) Call(_ super.Allocator, args []super.Value) super.Value {
	sVal := args[0]
	reVal := args[1]
	newVal := args[2]
	for i := range args {
		if !args[i].IsString() {
			return r.zctx.WrapError("regexp_replace: string arg required", args[i])
		}
	}
	if sVal.IsNull() {
		return super.Null
	}
	if reVal.IsNull() || newVal.IsNull() {
		return r.zctx.NewErrorf("regexp_replace: 2nd and 3rd args cannot be null")
	}
	if re := super.DecodeString(reVal.Bytes()); r.restr != re {
		r.restr = re
		r.re, r.err = regexp.Compile(re)
	}
	if r.err != nil {
		return r.zctx.NewErrorf("regexp_replace: %s", r.err)
	}
	return super.NewString(string(r.re.ReplaceAll(sVal.Bytes(), newVal.Bytes())))
}

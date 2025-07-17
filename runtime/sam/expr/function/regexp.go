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
	sctx    *super.Context
}

func (r *Regexp) Call(args []super.Value) super.Value {
	if !args[0].IsString() {
		return r.sctx.WrapError("regexp: string required for first arg", args[0])
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
		return r.sctx.WrapError(msg, args[0])
	}
	if !args[1].IsString() {
		return r.sctx.WrapError("regexp: string required for second arg", args[1])
	}
	r.builder.Reset()
	for _, b := range r.re.FindSubmatch(args[1].Bytes()) {
		r.builder.Append(b)
	}
	if r.typ == nil {
		r.typ = r.sctx.LookupTypeArray(super.TypeString)
	}
	return super.NewValue(r.typ, r.builder.Bytes())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#regexp_replace
type RegexpReplace struct {
	sctx  *super.Context
	re    *regexp.Regexp
	restr string
	err   error
}

func (r *RegexpReplace) Call(args []super.Value) super.Value {
	sVal := args[0].Under()
	reVal := args[1].Under()
	newVal := args[2].Under()
	for i := range args {
		if !args[i].IsString() {
			return r.sctx.WrapError("regexp_replace: string arg required", args[i])
		}
	}
	if sVal.IsNull() || reVal.IsNull() || newVal.IsNull() {
		return super.NullString
	}
	if re := super.DecodeString(reVal.Bytes()); r.restr != re {
		r.restr = re
		r.re, r.err = regexp.Compile(re)
	}
	if r.err != nil {
		msg := "regexp_replace: invalid regular expression"
		if syntaxErr, ok := r.err.(*syntax.Error); ok {
			msg += ": " + syntaxErr.Code.String()
		}
		return r.sctx.WrapError(msg, args[1])
	}
	return super.NewString(string(r.re.ReplaceAll(sVal.Bytes(), newVal.Bytes())))
}

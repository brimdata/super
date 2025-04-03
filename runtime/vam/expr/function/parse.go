package function

import (
	"strings"

	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio/supio"
)

type ParseURI struct {
	zctx  *super.Context
	samfn *samfunc.ParseURI
}

func newParseURI(zctx *super.Context) *ParseURI {
	return &ParseURI{zctx, samfunc.NewParseURI(zctx)}
}

func (p *ParseURI) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if vec.Type().ID() != super.IDString {
		return vector.NewWrappedError(p.zctx, "parse_uri: string arg required", args[0])
	}
	var b zcode.Builder
	db := vector.NewDynamicBuilder()
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		val := super.NewValue(super.TypeString, b.Bytes().Body())
		db.Write(p.samfn.Call(nil, []super.Value{val}))
	}
	return db.Build()
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#parse_sup
type ParseSUP struct {
	zctx *super.Context
	sr   *strings.Reader
	zr   *supio.Reader
}

func newParseSUP(zctx *super.Context) *ParseSUP {
	var sr strings.Reader
	return &ParseSUP{zctx, &sr, supio.NewReader(zctx, &sr)}
}

func (p *ParseSUP) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if vec.Type().ID() != super.IDString {
		return vector.NewWrappedError(p.zctx, "parse_sup: string arg required", args[0])
	}
	var errs []uint32
	errMsgs := vector.NewStringEmpty(0, nil)
	builder := vector.NewDynamicBuilder()
	for i := range vec.Len() {
		s, null := vector.StringValue(vec, i)
		if null {
			builder.Write(super.Null)
			continue
		}
		p.sr.Reset(s)
		val, err := p.zr.Read()
		if err != nil {
			errs = append(errs, i)
			errMsgs.Append("parse_sup: " + err.Error())
			continue
		}
		if val == nil {
			builder.Write(super.Null)
		} else {
			builder.Write(*val)
		}
	}
	out := builder.Build()
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewVecWrappedError(p.zctx, errMsgs, vector.NewView(args[0], errs)))
	}
	return out
}

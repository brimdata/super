package function

import (
	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
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

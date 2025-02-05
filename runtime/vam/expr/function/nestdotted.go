package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type NestDotted struct {
	zctx *super.Context
}

func (n *NestDotted) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[len(args)-1])
	if vec.Type().ID() == super.IDNull {
		return vec
	}
	view, ok := vec.(*vector.View)
	if ok {
		vec = view.Any
	}
	record, ok := vec.(*vector.Record)
	if !ok {
		return vector.NewWrappedError(n.zctx, "nest_dotted: non-record value", args[len(args)-1])
	}
	b, err := n.getBuilder(record.Typ)
	if err != nil {
		return vector.NewWrappedError(n.zctx, "nest_dotted: "+err.Error(), args[len(args)-1])
	}
	if b == nil {
		return args[len(args)-1]
	}
	out := vector.Any(b.New(record.Fields, record.Nulls))
	if view != nil {
		out = vector.NewView(out, view.Index)
	}
	return out
}

func (n *NestDotted) getBuilder(in *super.TypeRecord) (*vector.RecordBuilder, error) {
	var foundDotted bool
	var fields field.List
	for _, f := range in.Fields {
		dotted := field.Dotted(f.Name)
		if len(dotted) > 1 {
			foundDotted = true
		}
		fields = append(fields, dotted)
	}
	if !foundDotted {
		return nil, nil
	}
	return vector.NewRecordBuilder(n.zctx, fields)
}

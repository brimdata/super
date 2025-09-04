package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
)

type NestDotted struct {
	sctx        *super.Context
	builders    map[int]*super.RecordBuilder
	recordTypes map[int]*super.TypeRecord
}

// NewNestDotted returns a function that turns successive dotted
// field names into nested records.  For example, unflattening {"a.a":
// 1, "a.b": 1} results in {a:{a:1,b:1}}.  Note that while
// unflattening is applied recursively from the top-level and applies
// to arbitrary-depth dotted names, it is not applied to dotted names
// that start at lower levels (for example {a:{"a.a":1}} is
// unchanged).
func NewNestDotted(sctx *super.Context) *NestDotted {
	return &NestDotted{
		sctx:        sctx,
		builders:    make(map[int]*super.RecordBuilder),
		recordTypes: make(map[int]*super.TypeRecord),
	}
}

func (n *NestDotted) lookupBuilderAndType(in *super.TypeRecord) (*super.RecordBuilder, *super.TypeRecord, error) {
	if b, ok := n.builders[in.ID()]; ok {
		return b, n.recordTypes[in.ID()], nil
	}
	var foundDotted bool
	var fields field.List
	var types []super.Type
	for _, f := range in.Fields {
		dotted := field.Dotted(f.Name)
		if len(dotted) > 1 {
			foundDotted = true
		}
		fields = append(fields, dotted)
		types = append(types, f.Type)
	}
	if !foundDotted {
		return nil, nil, nil
	}
	b, err := super.NewRecordBuilder(n.sctx, fields)
	if err != nil {
		return nil, nil, err
	}
	typ := b.Type(types)
	n.builders[in.ID()] = b
	n.recordTypes[in.ID()] = typ
	return b, typ, nil
}

func (n *NestDotted) Call(args []super.Value) super.Value {
	val := args[len(args)-1]
	if val.Type().ID() == super.IDNull {
		return val
	}
	rtyp := super.TypeRecordOf(val.Type())
	if rtyp == nil {
		return n.sctx.WrapError("nest_dotted: non-record value", val)
	}
	b, typ, err := n.lookupBuilderAndType(rtyp)
	if err != nil {
		return n.sctx.WrapError("nest_dotted: "+err.Error(), val)
	}
	if b == nil {
		return val
	}
	if val.IsNull() {
		return super.NewValue(typ, nil)
	}
	b.Reset()
	for it := val.Bytes().Iter(); !it.Done(); {
		b.Append(it.Next())
	}
	zbytes, err := b.Encode()
	if err != nil {
		panic(err)
	}
	return super.NewValue(typ, zbytes)
}

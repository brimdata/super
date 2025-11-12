package explode

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
)

// A an explode Proc is a proc that, given an input record and a
// type T, outputs one record for each field of the input record of
// type T. It is useful for type-based indexing.
type Op struct {
	parent  sbuf.Puller
	outType super.Type
	typ     super.Type
	args    []expr.Evaluator
}

// New creates a exploder for type typ, where the
// output records' single field is named name.
func New(sctx *super.Context, parent sbuf.Puller, args []expr.Evaluator, typ super.Type, name string) (sbuf.Puller, error) {
	return &Op{
		parent:  parent,
		outType: sctx.MustLookupTypeRecord([]super.Field{{Name: name, Type: typ}}),
		typ:     typ,
		args:    args,
	}, nil
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	for {
		batch, err := o.parent.Pull(done)
		if batch == nil || err != nil {
			return nil, err
		}
		vals := batch.Values()
		out := make([]super.Value, 0, len(vals))
		for i := range vals {
			for _, arg := range o.args {
				val := arg.Eval(vals[i])
				if val.IsError() {
					if !val.IsMissing() {
						out = append(out, val.Copy())
					}
					continue
				}
				super.Walk(val.Type(), val.Bytes(), func(typ super.Type, body scode.Bytes) error {
					if typ == o.typ && body != nil {
						bytes := scode.Append(nil, body)
						out = append(out, super.NewValue(o.outType, bytes))
						return super.SkipContainer
					}
					return nil
				})
			}
		}
		if len(out) > 0 {
			defer batch.Unref()
			return sbuf.NewBatch(out), nil
		}
		batch.Unref()
	}
}

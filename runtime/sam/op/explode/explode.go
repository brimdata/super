package explode

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

// A an explode Proc is a proc that, given an input record and a
// type T, outputs one record for each field of the input record of
// type T. It is useful for type-based indexing.
type Op struct {
	parent   zbuf.Puller
	outType  super.Type
	typ      super.Type
	args     []expr.Evaluator
	resetter expr.Resetter
}

// New creates a exploder for type typ, where the
// output records' single field is named name.
func New(sctx *super.Context, parent zbuf.Puller, args []expr.Evaluator, typ super.Type, name string, resetter expr.Resetter) (zbuf.Puller, error) {
	return &Op{
		parent:   parent,
		outType:  sctx.MustLookupTypeRecord([]super.Field{{Name: name, Type: typ}}),
		typ:      typ,
		args:     args,
		resetter: resetter,
	}, nil
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	for {
		batch, err := o.parent.Pull(done)
		if batch == nil || err != nil {
			o.resetter.Reset()
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
				super.Walk(val.Type(), val.Bytes(), func(typ super.Type, body zcode.Bytes) error {
					if typ == o.typ && body != nil {
						bytes := zcode.Append(nil, body)
						out = append(out, super.NewValue(o.outType, bytes))
						return super.SkipContainer
					}
					return nil
				})
			}
		}
		if len(out) > 0 {
			defer batch.Unref()
			return zbuf.NewBatch(out), nil
		}
		batch.Unref()
	}
}

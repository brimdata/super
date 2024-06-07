package explode

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/sam/expr"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zcode"
)

// A an explode Proc is a proc that, given an input record and a
// zng type T, outputs one record for each field of the input record of
// type T. It is useful for type-based indexing.
type Op struct {
	parent   zbuf.Puller
	outType  zed.Type
	typ      zed.Type
	args     []expr.Evaluator
	resetter expr.Resetter
}

// New creates a exploder for type typ, where the
// output records' single field is named name.
func New(zctx *zed.Context, parent zbuf.Puller, args []expr.Evaluator, typ zed.Type, name string, resetter expr.Resetter) (zbuf.Puller, error) {
	return &Op{
		parent:   parent,
		outType:  zctx.MustLookupTypeRecord([]zed.Field{{Name: name, Type: typ}}),
		typ:      typ,
		args:     args,
		resetter: resetter,
	}, nil
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	arena := zed.NewArena()
	defer arena.Unref()
	for {
		batch, err := o.parent.Pull(done)
		if batch == nil || err != nil {
			o.resetter.Reset()
			return nil, err
		}
		ectx := expr.NewContextWithVars(arena, batch.Vars())
		vals := batch.Values()
		out := make([]zed.Value, 0, len(vals))
		for _, val := range vals {
			for _, arg := range o.args {
				val := arg.Eval(ectx, val)
				if val.IsError() {
					if !val.IsMissing() {
						out = append(out, val)
					}
					continue
				}
				zed.Walk(val.Type(), val.Bytes(), func(typ zed.Type, body zcode.Bytes) error {
					if typ == o.typ && body != nil {
						bytes := zcode.Append(nil, body)
						out = append(out, arena.New(o.outType, bytes))
						return zed.SkipContainer
					}
					return nil
				})
			}
		}
		if len(out) > 0 {
			defer batch.Unref()
			return zbuf.NewBatch(arena, out, batch, batch.Vars()), nil
		}
		arena.Reset()
		batch.Unref()
	}
}

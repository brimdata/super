package explode

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/proc"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zcode"
)

// A an explode Proc is a proc that, given an input record and a
// zng type T, outputs one record for each field of the input record of
// type T. It is useful for type-based indexing.
type Proc struct {
	parent  proc.Interface
	builder zed.Builder
	typ     zed.Type
	args    []expr.Evaluator
}

// New creates a exploder for type typ, where the
// output records' single column is named name.
func New(zctx *zed.Context, parent proc.Interface, args []expr.Evaluator, typ zed.Type, name string) (proc.Interface, error) {
	cols := []zed.Column{{Name: name, Type: typ}}
	rectyp := zctx.MustLookupTypeRecord(cols)
	builder := zed.NewBuilder(rectyp)
	return &Proc{
		parent:  parent,
		builder: *builder,
		typ:     typ,
		args:    args,
	}, nil
}

func (p *Proc) Pull() (zbuf.Batch, error) {
	for {
		batch, err := p.parent.Pull()
		if proc.EOS(batch, err) {
			return nil, err
		}
		recs := make([]*zed.Record, 0, batch.Length())
		for _, rec := range batch.Records() {
			for _, arg := range p.args {
				zv, err := arg.Eval(rec)
				if err != nil {
					return nil, err
				}
				zed.Walk(zv.Type, zv.Bytes, func(typ zed.Type, body zcode.Bytes) error {
					if typ == p.typ && body != nil {
						recs = append(recs, p.builder.Build(body).Keep())
						return zed.SkipContainer
					}
					return nil
				})
			}
		}
		batch.Unref()
		if len(recs) > 0 {
			return zbuf.Array(recs), nil
		}
	}
}

func (p *Proc) Done() {
	p.parent.Done()
}

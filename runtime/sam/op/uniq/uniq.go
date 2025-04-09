package uniq

import (
	"bytes"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

type Op struct {
	rctx    *runtime.Context
	parent  zbuf.Puller
	builder zcode.Builder
	cflag   bool
	count   uint64
	last    *super.Value
	eos     bool
}

func New(rctx *runtime.Context, parent zbuf.Puller, cflag bool) *Op {
	return &Op{
		rctx:   rctx,
		parent: parent,
		cflag:  cflag,
	}
}

func (o *Op) wrap(t *super.Value) super.Value {
	if o.cflag {
		o.builder.Reset()
		o.builder.Append(t.Bytes())
		o.builder.Append(super.EncodeUint(o.count))
		typ := o.rctx.Sctx.MustLookupTypeRecord([]super.Field{
			super.NewField("value", t.Type()),
			super.NewField("count", super.TypeUint64),
		})
		return super.NewValue(typ, o.builder.Bytes()).Copy()
	}
	return *t
}

func (o *Op) appendUniq(out []super.Value, t *super.Value) []super.Value {
	if o.count == 0 {
		o.last = t.Copy().Ptr()
		o.count = 1
		return out
	} else if bytes.Equal(t.Bytes(), o.last.Bytes()) {
		o.count++
		return out
	}
	out = append(out, o.wrap(o.last))
	o.last = t.Copy().Ptr()
	o.count = 1
	return out
}

// uniq is a little bit complicated because we have to check uniqueness
// across records between calls to Pull.
func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.eos {
		o.eos = false
		return nil, nil
	}
	for {
		batch, err := o.parent.Pull(done)
		if err != nil {
			return nil, err
		}
		if batch == nil {
			if o.last == nil {
				return nil, nil
			}
			o.eos = true
			t := o.wrap(o.last)
			o.last = nil
			return zbuf.NewArray([]super.Value{t}), nil
		}
		var out []super.Value
		vals := batch.Values()
		for i := range vals {
			out = o.appendUniq(out, &vals[i])
		}
		batch.Unref()
		if len(out) > 0 {
			return zbuf.NewArray(out), nil
		}
	}
}

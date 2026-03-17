package infer

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sbuf"
)

type Op struct {
	rctx      *runtime.Context
	parent    sbuf.Puller
	converter *converter
	limit     int
	needEOS   bool
}

func New(rctx *runtime.Context, parent sbuf.Puller, limit int) *Op {
	return &Op{
		rctx:   rctx,
		parent: parent,
		limit:  limit,
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	if done {
		o.eos()
		return o.parent.Pull(true)
	}
	if o.needEOS {
		o.eos()
		return nil, nil
	}
	if o.converter == nil {
		o.converter = newConverter(o.rctx, o.limit)
	}
	for {
		batch, err := o.parent.Pull(false)
		if err != nil {
			o.eos()
			return nil, err
		}
		if batch == nil {
			batch, err := o.converter.finish()
			if err != nil {
				o.eos()
				return nil, err
			}
			if batch != nil {
				o.needEOS = true
			} else {
				o.eos()
			}
			return batch, nil
		}
		batch, err = o.converter.process(batch)
		if err != nil {
			o.eos()
			return nil, err
		}
		if batch != nil {
			return batch, nil
		}
	}
}

func (o *Op) eos() {
	o.converter = nil
	o.needEOS = false
}

type converter struct {
	rctx   *runtime.Context
	queues map[super.Type][]super.Value
	caster function.Caster
	target map[super.Type]super.Type
	limit  int
}

func newConverter(rctx *runtime.Context, limit int) *converter {
	return &converter{
		rctx:   rctx,
		queues: make(map[super.Type][]super.Value),
		caster: function.NewCaster(rctx.Sctx),
		target: make(map[super.Type]super.Type),
		limit:  limit,
	}
}

func (c *converter) process(batch sbuf.Batch) (sbuf.Batch, error) {
	var out sbuf.Array
	for _, val := range batch.Values() {
		if val, ok := c.convert(val); ok {
			out.Append(val)
		}
	}
	batch.Unref()
	if err := c.drain(&out, false); err != nil {
		return nil, err
	}
	if len(out.Values()) != 0 {
		return &out, nil
	}
	return nil, nil
}

func (c *converter) drain(out *sbuf.Array, force bool) error {
	for typ, q := range c.queues {
		// The queues can get big, so we mind the context.
		if err := c.rctx.Err(); err != nil {
			return err
		}
		if force || (c.limit != 0 && len(q) >= c.limit) {
			c.infer(q)
			delete(c.queues, typ)
			for _, val := range q {
				val, ok := c.convert(val)
				if !ok {
					panic(c)
				}
				out.Append(val)
			}
		}
	}
	return nil
}

func (c *converter) convert(val super.Value) (super.Value, bool) {
	if to, ok := c.target[val.Type()]; ok {
		if to != nil {
			if converted, ok := c.caster.Cast(val, to); ok {
				return converted, true
			}
			return c.rctx.Sctx.WrapError("inference cast failed (try larger sample size)", val), true
		}
		return val, true
	}
	if c.enq(val) {
		return val, true
	}
	return super.Value{}, false
}

func (c *converter) finish() (sbuf.Batch, error) {
	var out sbuf.Array
	if err := c.drain(&out, true); err != nil {
		return nil, err
	}
	if len(out.Values()) != 0 {
		return &out, nil
	}
	return nil, nil
}

func (c *converter) enq(val super.Value) bool {
	typ := val.Type()
	q, ok := c.queues[typ]
	if !ok {
		if newInferer(typ) == nil {
			// No string fields... skip
			c.target[typ] = nil
			return true
		}
		c.queues[typ] = make([]super.Value, 0, c.limit)
	}
	c.queues[typ] = append(q, val.Copy())
	return false
}

func (c *converter) infer(q []super.Value) {
	typ := q[0].Type()
	infer := newInferer(typ)
	vals := q
	if c.limit != 0 && len(vals) >= c.limit {
		vals = vals[:c.limit]
	}
	for _, val := range vals {
		infer.load(typ, val.Bytes())
	}
	c.target[typ] = infer.typeof(c.rctx.Sctx, typ)
}

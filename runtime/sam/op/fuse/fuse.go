package fuse

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/runtime/sam/op/spill"
	"github.com/brimdata/super/sbuf"
)

var MemMaxBytes = 128 * 1024 * 1024

type Op struct {
	rctx   *runtime.Context
	parent sbuf.Puller

	fuser    *valueFuser
	once     sync.Once
	resultCh chan op.Result
}

func New(rctx *runtime.Context, parent sbuf.Puller) *Op {
	return &Op{
		rctx:     rctx,
		parent:   parent,
		fuser:    newValueFuser(rctx.Sctx, MemMaxBytes),
		resultCh: make(chan op.Result),
	}
}

func (o *Op) Pull(done bool) (sbuf.Batch, error) {
	// XXX ignoring the done indicator.  See issue #3436.
	o.once.Do(func() { go o.run() })
	if r, ok := <-o.resultCh; ok {
		return r.Batch, r.Err
	}
	return nil, o.rctx.Err()
}

func (o *Op) run() {
	if err := o.pullInput(); err != nil {
		o.shutdown(err)
		return
	}
	o.shutdown(o.pushOutput())
}

func (o *Op) pullInput() error {
	for {
		if err := o.rctx.Err(); err != nil {
			return err
		}
		batch, err := o.parent.Pull(false)
		if err != nil {
			return err
		}
		if batch == nil {
			return nil
		}
		if err := sbuf.WriteBatch(o.fuser, batch); err != nil {
			return err
		}
		batch.Unref()
	}
}

func (o *Op) pushOutput() error {
	puller := sbuf.NewPuller(o.fuser)
	for {
		if err := o.rctx.Err(); err != nil {
			return err
		}
		batch, err := puller.Pull(false)
		if err != nil || batch == nil {
			return err
		}
		o.sendResult(batch, nil)
	}
}

func (o *Op) sendResult(b sbuf.Batch, err error) {
	select {
	case o.resultCh <- op.Result{Batch: b, Err: err}:
	case <-o.rctx.Done():
	}
}

func (o *Op) shutdown(err error) {
	if err2 := o.fuser.Close(); err == nil {
		err = err2
	}
	o.sendResult(nil, err)
	close(o.resultCh)
}

// valueFuser buffers values, computes a supertype over all the values,
// then upcasts the values to the computed supertype as the values are read.
type valueFuser struct {
	sctx        *super.Context
	memMaxBytes int

	nbytes  int
	vals    []super.Value
	spiller *spill.File

	fuser  *agg.Fuser
	caster function.Caster
	typ    super.Type
}

// newValueFuser returns a new valueFuser that buffers values in memory until
// their cumulative size (measured in scode.Bytes length) exceeds memMaxBytes,
// at which point it buffers them in a temporary file.
func newValueFuser(sctx *super.Context, memMaxBytes int) *valueFuser {
	return &valueFuser{
		sctx:        sctx,
		memMaxBytes: memMaxBytes,
		fuser:       agg.NewFuserWithMissingFieldsAsNullable(sctx),
		caster:      function.NewUpCaster(sctx),
	}
}

// Close removes the receiver's temporary file if it created one.
func (v *valueFuser) Close() error {
	if v.spiller != nil {
		return v.spiller.CloseAndRemove()
	}
	return nil
}

// Write buffers rec. If called after Read, Write panics.
func (v *valueFuser) Write(val super.Value) error {
	if v.typ != nil {
		panic("fuser: write after read")
	}
	v.fuser.Fuse(val.Type())
	if v.spiller != nil {
		return v.spiller.Write(val)
	}
	return v.stash(val)
}

func (v *valueFuser) stash(val super.Value) error {
	v.nbytes += len(val.Bytes())
	if v.nbytes >= v.memMaxBytes {
		var err error
		v.spiller, err = spill.NewTempFile()
		if err != nil {
			return err
		}
		for _, val := range v.vals {
			if err := v.spiller.Write(val); err != nil {
				return err
			}
		}
		v.vals = nil
		return v.spiller.Write(val)
	}
	v.vals = append(v.vals, val.Copy())
	return nil
}

// Read returns the next buffered value after upcasting to the supertype.
func (v *valueFuser) Read() (*super.Value, error) {
	if v.typ == nil {
		v.typ = v.fuser.Type()
		if v.spiller != nil {
			if err := v.spiller.Rewind(v.sctx); err != nil {
				return nil, err
			}
		}
	}
	val, err := v.next()
	if val == nil || err != nil {
		return nil, err
	}
	return v.caster.Cast(*val, v.typ).Ptr(), nil
}

func (v *valueFuser) next() (*super.Value, error) {
	if v.spiller != nil {
		return v.spiller.Read()
	}
	var val *super.Value
	if len(v.vals) > 0 {
		val = &v.vals[0]
		v.vals = v.vals[1:]
	}
	return val, nil
}

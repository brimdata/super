package fuse

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/sam/op/spill"
)

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

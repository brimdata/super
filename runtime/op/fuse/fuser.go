package fuse

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/runtime/expr/agg"
	"github.com/brimdata/zed/runtime/op/spill"
)

// Fuser buffers records written to it, assembling from them a unified schema of
// fields and types.  Fuser then transforms those records to the unified schema
// as they are read back from it.
type Fuser struct {
	zctx        *zed.Context
	memMaxBytes int

	nbytes  int
	vals    []*zed.Value
	spiller *spill.File

	types      map[zed.Type]struct{}
	uberSchema *agg.Schema
	shaper     *expr.ConstShaper
	ectx       expr.ResetContext
}

// NewFuser returns a new Fuser.  The Fuser buffers records in memory until
// their cumulative size (measured in zcode.Bytes length) exceeds memMaxBytes,
// at which point it buffers them in a temporary file.
func NewFuser(zctx *zed.Context, memMaxBytes int) *Fuser {
	return &Fuser{
		zctx:        zctx,
		memMaxBytes: memMaxBytes,
		types:       make(map[zed.Type]struct{}),
		uberSchema:  agg.NewSchema(zctx),
	}
}

// Close removes the receiver's temporary file if it created one.
func (f *Fuser) Close() error {
	if f.spiller != nil {
		return f.spiller.CloseAndRemove()
	}
	return nil
}

// Write buffers rec. If called after Read, Write panics.
func (f *Fuser) Write(rec *zed.Value) error {
	if f.shaper != nil {
		panic("fuser: write after read")
	}
	if _, ok := f.types[rec.Type()]; !ok {
		f.types[rec.Type()] = struct{}{}
		f.uberSchema.Mixin(rec.Type())
	}
	if f.spiller != nil {
		return f.spiller.Write(rec)
	}
	return f.stash(rec)
}

func (f *Fuser) stash(rec *zed.Value) error {
	f.nbytes += len(rec.Bytes())
	if f.nbytes >= f.memMaxBytes {
		var err error
		f.spiller, err = spill.NewTempFile()
		if err != nil {
			return err
		}
		for _, rec := range f.vals {
			if err := f.spiller.Write(rec); err != nil {
				return err
			}
		}
		f.vals = nil
		return f.spiller.Write(rec)
	}
	f.vals = append(f.vals, rec.Copy())
	return nil
}

// Read returns the next buffered record after transforming it to the unified
// schema.
func (f *Fuser) Read() (*zed.Value, error) {
	if f.shaper == nil {
		t := f.uberSchema.Type()
		f.shaper = expr.NewConstShaper(f.zctx, &expr.This{}, t, expr.Cast|expr.Fill|expr.Order)
		if f.spiller != nil {
			if err := f.spiller.Rewind(f.zctx); err != nil {
				return nil, err
			}
		}
	}
	rec, err := f.next()
	if rec == nil || err != nil {
		return nil, err
	}
	return f.shaper.Eval(f.ectx.Reset(), rec), nil
}

func (f *Fuser) next() (*zed.Value, error) {
	if f.spiller != nil {
		return f.spiller.Read()
	}
	var rec *zed.Value
	if len(f.vals) > 0 {
		rec = f.vals[0]
		f.vals = f.vals[1:]
	}
	return rec, nil

}

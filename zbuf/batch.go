package zbuf

import (
	"sync"
	"sync/atomic"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zio"
	"golang.org/x/exp/slices"
)

// Batch is an interface to a bundle of values.  Reference counting allows
// efficient, safe reuse in concert with sharing across goroutines.
//
// A newly obtained Batch always has a reference count of one.  The Batch owns
// its values and their storage, and an implementation may reuse this memory
// when the reference count falls to zero, reducing load on the garbage
// collector.
//
// To promote reuse, a goroutine should release a Batch reference when possible,
// but care must be taken to avoid race conditions that arise from releasing a
// reference too soon.  Specifically, a goroutine obtaining a value from a
// Batch must retain its Batch reference for as long as it retains the value,
// and the goroutine must not use the value after releasing its reference.
//
// Regardless of reference count or implementation, an unreachable Batch will
// eventually be reclaimed by the garbage collector.
type Batch interface {
	Ref()
	Unref()
	Values() []zed.Value
	// Vars accesses the variables reachable in the current scope.
	Vars() []zed.Value
}

// WriteBatch writes the values in batch to zw.  If an error occurs, WriteBatch
// stops and returns the error.
func WriteBatch(zw zio.Writer, batch Batch) error {
	vals := batch.Values()
	for i := range vals {
		if err := zw.Write(&vals[i]); err != nil {
			return err
		}
	}
	return nil
}

// A Puller produces Batches of records, signaling end-of-stream (EOS) by returning
// a nil Batch and nil error.  The done argument to Pull indicates that the stream
// should be terminated before its natural EOS.  An implementation must return
// EOS in response to a Pull call when the done parameter is true.  After seeing EOS,
// (either via done or its natural end), an implementation of an operator that
// processes pulled data should respond to additional calls to Pull as if restarting
// in its initial state.  For original sources of data (e.g., the from operator),
// once EOS is reached, additional calls to Pull after the first EOS should always
// return EOS.  Pull is not safe to call concurrently.
type Puller interface {
	Pull(bool) (Batch, error)
}

// PullerBatchBytes is the maximum number of bytes (in the zed.Value.Byte
// sense) per batch for a [Puller] created by [NewPuller].
const PullerBatchBytes = 512 * 1024

// PullerBatchValues is the maximum number of values per batch for a [Puller]
// created by [NewPuller].
var PullerBatchValues = 100

// NewPuller returns a puller for zr that returns batches containing up to
// [PullerBatchBytes] bytes and [PullerBatchValues] values.
func NewPuller(zr zio.Reader) Puller {
	return &puller{zr}
}

type puller struct {
	zr zio.Reader
}

func (p *puller) Pull(bool) (Batch, error) {
	if p.zr == nil {
		return nil, nil
	}
	batch := newPullerBatch()
	for {
		val, err := p.zr.Read()
		if err != nil {
			return nil, err
		}
		if val == nil {
			p.zr = nil
			if len(batch.vals) == 0 {
				return nil, nil
			}
			return batch, nil
		}
		if batch.appendVal(val) {
			return batch, nil
		}
	}
}

type pullerBatch struct {
	buf  []byte
	refs atomic.Int32
	vals []zed.Value
}

var pullerBatchPool sync.Pool

func newPullerBatch() *pullerBatch {
	b, ok := pullerBatchPool.Get().(*pullerBatch)
	if !ok {
		b = &pullerBatch{
			buf:  make([]byte, PullerBatchBytes),
			vals: make([]zed.Value, PullerBatchValues),
		}
	}
	b.buf = b.buf[:0]
	b.refs.Store(1)
	b.vals = b.vals[:0]
	return b
}

// appendVal appends a copy of val to b.  appendVal returns true if b is full
// (i.e., b.buf is full, b.buf had insufficient space for val.Bytes, or b.val is
// full).  appendVal never reallocates b.buf or b.vals.
func (b *pullerBatch) appendVal(val *zed.Value) bool {
	var bytes []byte
	var bufFull bool
	if !val.IsNull() {
		if avail := cap(b.buf) - len(b.buf); avail >= len(val.Bytes()) {
			// Append to b.buf since that won't reallocate.
			start := len(b.buf)
			b.buf = append(b.buf, val.Bytes()...)
			bytes = b.buf[start:]
			bufFull = avail == len(val.Bytes())
		} else {
			// Copy since appending to b.buf would reallocate.
			bytes = slices.Clone(val.Bytes())
			bufFull = true
		}
	}
	b.vals = append(b.vals, *zed.NewValue(val.Type, bytes))
	return bufFull || len(b.vals) == cap(b.vals)
}

func (b *pullerBatch) Ref() { b.refs.Add(1) }

func (b *pullerBatch) Unref() {
	if refs := b.refs.Add(-1); refs == 0 {
		pullerBatchPool.Put(b)
	} else if refs < 0 {
		panic("zbuf: negative batch reference count")
	}
}

func (p *pullerBatch) Values() []zed.Value { return p.vals }
func (*pullerBatch) Vars() []zed.Value     { return nil }

func CopyPuller(w zio.Writer, p Puller) error {
	for {
		b, err := p.Pull(false)
		if b == nil || err != nil {
			return err
		}
		if err := WriteBatch(w, b); err != nil {
			return err
		}
		b.Unref()
	}
}

func PullerReader(p Puller) zio.Reader {
	return &pullerReader{p: p}
}

type pullerReader struct {
	p     Puller
	batch Batch
	vals  []zed.Value
}

func (r *pullerReader) Read() (*zed.Value, error) {
	// Loop handles zero-length batches.
	for len(r.vals) == 0 {
		if r.batch != nil {
			r.batch.Unref()
			r.batch = nil
		}
		batch, err := r.p.Pull(false)
		if batch == nil || err != nil {
			return nil, err
		}
		r.batch = batch
		r.vals = batch.Values()
	}
	val := &r.vals[0]
	r.vals = r.vals[1:]
	return val, nil
}

// XXX at some point the stacked scopes should not make copies of values
// but merely refer back to the value in the wrapped batch, and we should
// ref the wrapped batch then downstream entities will unref it, but how
// do we carry the var frame through... protocol needs to be that any new
// batch created by a proc needs to preserve the var frame... we don't
// do that right now and ref counting needs to account for the dependencies.
// procs like summarize and sort that unref their input batches merely need
// to copy the first frame (of each batch) and the contract is that the
// frame will not change between multiple batches within a single-EOS event.

type batch struct {
	Batch
	vars []zed.Value
}

func NewBatch(b Batch, vals []zed.Value) Batch {
	return &batch{
		Batch: NewArray(vals),
		vars:  CopyVars(b),
	}
}

func (b *batch) Vars() []zed.Value {
	return b.vars
}

func CopyVars(b Batch) []zed.Value {
	vars := b.Vars()
	if len(vars) > 0 {
		newvars := make([]zed.Value, len(vars))
		for k, v := range vars {
			newvars[k] = *v.Copy()
		}
		vars = newvars
	}
	return vars
}

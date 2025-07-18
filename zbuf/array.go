package zbuf

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
)

// Array is a slice of of records that implements the Batch and
// the Reader interfaces.
type Array struct {
	values []super.Value
}

var _ Batch = (*Array)(nil)
var _ zio.Reader = (*Array)(nil)
var _ zio.Writer = (*Array)(nil)

// XXX this should take the frame arg too and the procs that create
// new arrays need to propagate their frames downstream.
func NewArray(vals []super.Value) *Array {
	return &Array{values: vals}
}

func (a *Array) Ref() {
	// do nothing... let the GC reclaim it
}

func (a *Array) Unref() {
	// do nothing... let the GC reclaim it
}

func (a *Array) Values() []super.Value {
	return a.values
}

func (a *Array) Append(r super.Value) {
	a.values = append(a.values, r)
}

func (a *Array) Write(r super.Value) error {
	a.Append(r.Copy())
	return nil
}

// Read returns removes the first element of the Array and returns it,
// or it returns nil if the Array is empty.
func (a *Array) Read() (*super.Value, error) {
	var rec *super.Value
	if len(a.values) > 0 {
		rec = &a.values[0]
		a.values = a.values[1:]
	}
	return rec, nil
}

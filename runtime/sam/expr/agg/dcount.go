package agg

import (
	"fmt"

	"github.com/axiomhq/hyperloglog"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

// DCount uses hyperloglog to approximate the count of unique values for
// a field.
type DCount struct {
	scratch scode.Bytes
	sketch  *hyperloglog.Sketch
}

var _ Function = (*DCount)(nil)

func NewDCount() *DCount {
	return &DCount{
		sketch: hyperloglog.New(),
	}
}

func (d *DCount) Consume(val super.Value) {
	if val.IsNull() {
		return
	}
	d.scratch = d.scratch[:0]
	// append type id to vals so we get a unique count where the bytes are same
	// but the super.Type is different.
	d.scratch = super.AppendInt(d.scratch, int64(val.Type().ID()))
	d.scratch = append(d.scratch, val.Bytes()...)
	d.sketch.Insert(d.scratch)
}

func (d *DCount) Result(*super.Context) super.Value {
	return super.NewUint64(d.sketch.Estimate())
}

func (d *DCount) ConsumeAsPartial(partial super.Value) {
	if partial.Type() != super.TypeBytes {
		panic(fmt.Errorf("dcount: partial has bad type: %s", sup.FormatValue(partial)))
	}
	var s hyperloglog.Sketch
	if err := s.UnmarshalBinary(partial.Bytes()); err != nil {
		panic(fmt.Errorf("dcount: unmarshaling partial: %w", err))
	}
	d.sketch.Merge(&s)
}

func (d *DCount) ResultAsPartial(sctx *super.Context) super.Value {
	b, err := d.sketch.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("dcount: marshaling partial: %w", err))
	}
	return super.NewBytes(b)
}

package agg

import (
	"fmt"

	"github.com/axiomhq/hyperloglog"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

// dcount uses hyperloglog to approximate the count of unique values for
// a field.
type dcount struct {
	sketch *hyperloglog.Sketch
}

func newDCount() *dcount {
	return &dcount{
		sketch: hyperloglog.New(),
	}
}

func (d *dcount) Consume(vec vector.Any) {
	// append type id to vals so we get a unique count where the bytes are same
	// but the super.Type is different.
	scratch := super.AppendInt(nil, int64(vec.Type().ID()))
	var b scode.Builder
	for i := range vec.Len() {
		b.Truncate()
		vec.Serialize(&b, i)
		if vb := b.Bytes().Body(); vb != nil {
			d.sketch.Insert(append(scratch, vb...))
		}
	}
}

func (d *dcount) Result(*super.Context) super.Value {
	return super.NewInt64(int64(d.sketch.Estimate()))
}

func (d *dcount) ConsumeAsPartial(partial vector.Any) {
	if partial.Type() != super.TypeBytes || partial.Len() != 1 {
		panic("dcount: invalid partial")
	}
	b, _ := vector.BytesValue(partial, 0)
	var s hyperloglog.Sketch
	if err := s.UnmarshalBinary(b); err != nil {
		panic(fmt.Errorf("dcount: unmarshaling partial: %w", err))
	}
	d.sketch.Merge(&s)
}

func (d *dcount) ResultAsPartial(sctx *super.Context) super.Value {
	b, err := d.sketch.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("dcount: marshaling partial: %w", err))
	}
	return super.NewBytes(b)
}

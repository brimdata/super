package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Record struct {
	Typ    *super.TypeRecord
	Fields []Any
	len    uint32
}

var _ Any = (*Record)(nil)

func NewRecord(typ *super.TypeRecord, fields []Any, length uint32) *Record {
	return &Record{typ, fields, length}
}

func (*Record) Kind() Kind {
	return KindRecord
}

func (r *Record) Type() super.Type {
	return r.Typ
}

func (r *Record) Len() uint32 {
	return r.len
}

func (r *Record) ChangeType(typ *super.TypeRecord) *Record {
	fields := slices.Clone(r.Fields)
	for i, f := range typ.Fields {
		if rtyp, ok := f.Type.(*super.TypeRecord); ok {
			fields[i] = r.Fields[i].(*Record).ChangeType(rtyp)
		}
	}
	return &Record{typ, fields, r.len}
}

func (r *Record) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	for _, f := range r.Fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}

func buildTags(runlens []uint32, n uint32) ([]uint32, uint32) {
	tags := make([]uint32, n)
	off := 0
	var noneLen uint32
	for in := 0; in < len(runlens); {
		noneRun := runlens[in]
		in++
		for k := range int(noneRun) {
			tags[off+k] = 1
		}
		off += int(noneRun)
		noneLen += noneRun
		if in >= len(runlens) {
			break
		}
		// skip over values (leaving tags 0)
		off += int(runlens[in])
		in++
	}
	return tags, noneLen
}

// RLE emits a sequence of runs of the length of alternating sequences
// of nones and values, beginning with nones.  Every run is non-zero except for
// the first, which may be zero when the first value is non-none.
type RLE struct {
	runs       []uint32
	prediction uint32
	last       uint32
}

// Touch is called for each offset at which a value occurs.
// From this, we derive the runs of values and nones interleaved beginning
// with the first run of nones (which may be 0).
// Whenever there is a gap in values, we record the gap size as a run.
// When touch is called consecutively, we wait for for a gap before
// recording the none run immediately followed by the gap.
func (r *RLE) Touch(off uint32) {
	if r.prediction == r.last {
		// This happens only on first call.
		// Emit length of none run.
		r.emit(off)
		r.last = off
	} else if r.prediction != off {
		// emit length of value run
		r.emit(r.prediction - r.last)
		// emit length of none run
		r.emit(off - r.prediction)
		r.last = off
	}
	r.prediction = off + 1
}

func (r *RLE) End(off uint32) []uint32 {
	if r.prediction == r.last {
		// all nones
		r.emit(off)
	} else if r.prediction == off {
		if len(r.runs) == 1 && r.runs[0] == 0 {
			// all values
			return nil
		}
		// write the last run of values
		r.emit(off - r.last)
	} else {
		// write the last run of values and the last run of nones
		r.Touch(off)
	}
	return r.runs
}

func (r *RLE) emit(run uint32) {
	r.runs = append(r.runs, run)
}

// XXX
func xxxisNone(vec Any, slot uint32) bool {
	if _, ok := vec.(*None); ok {
		return true
	}
	if o, ok := vec.(*Optional); ok {
		return o.Dynamic.Tags[slot] == 1
	}
	return false
}

func NewOptionFromRLE(sctx *super.Context, vec Any, length uint32, runlens []uint32) Any {
	if len(runlens) == 0 {
		return vec
	}
	tags, noneLen := buildTags(runlens, length)
	if noneLen == 0 {
		// This value is optional but everything is here in this instance.
		return vec
	}
	//XXX right now we implement the none piece of an optional as a union inside of dynamic
	// where the none vector carries the type of value vector. I think we need this to
	// support vector record expressions where you have a none component and need to
	// carry the type in the union... but is there a better way where the type info
	// can just be in the top-level dynamic?  XXX Need to dig into this more.
	return &Optional{Dynamic: NewDynamic(tags, []Any{vec, NewNoneOption(sctx, vec.Type(), noneLen)})}
}

// An Optional value is a special Dynamic that has two tags comprising the
// values present and the Nones. XXX the tags may be run-length encoded to
// efficiently support a large collection of fused fields in a record without
// blowing open the tags for every column especially when the field is not
// accessed.  XXX get it working first without loader, the add loader concept
// to load the tags dynamically.
type Optional struct {
	typ *super.TypeUnion //XXX
	*Dynamic
}

// XXX this should return the option type union
func (o *Optional) Type() super.Type {
	//XXX explain why this is not problematic
	return o.Dynamic.Values[0].Type()
}

func (f *Optional) RLE() []uint32 {
	var rle RLE
	for slot := range f.Len() {
		// Touch all the values
		if f.Tags[slot] == 0 {
			rle.Touch(slot)
		}
	}
	return rle.End(f.Len())
}

func Deoption(sctx *super.Context, vec Any) Any {
	switch vec := vec.(type) {
	case *Optional:
		return vec.Dynamic
	case *None:
		return NewMissing(sctx, vec.len)
	}
	return vec
}

// XXX change name, move to apply
func DeoptionRaw(vec Any) Any {
	switch vec := vec.(type) {
	case *Optional:
		return vec.Dynamic
	}
	return vec
}

package vector

import (
	"slices"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Record struct {
	Typ    *super.TypeRecord
	fields []*Field
	len    uint32
}

var _ Any = (*Record)(nil)

func NewRecord(typ *super.TypeRecord, fields []Any, length uint32) *Record {
	wrapper := make([]*Field, 0, len(fields))
	for _, f := range fields {
		wrapper = append(wrapper, &Field{Val: f, Len: length})
	}
	return &Record{typ, wrapper, length}
}

func NewRecordFromFields(typ *super.TypeRecord, fields []*Field, length uint32) *Record {
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

func (r *Record) Fields(sctx *super.Context) []Any {
	val := make([]Any, 0, len(r.fields))
	for k := range r.fields {
		if len(r.fields[k].Runs) != 0 {
			val = append(val, r.fields[k].Deref(sctx))
		} else {
			val = append(val, r.fields[k].Val)
		}
	}
	return val
}

func (r *Record) Field(i int) *Field {
	return r.fields[i]
}

func (r *Record) Slice(from, to int) []*Field {
	return r.fields[from:to]
}

func (r *Record) ChangeType(typ *super.TypeRecord) *Record {
	fields := slices.Clone(r.fields)
	for i, f := range typ.Fields {
		if rtyp, ok := f.Type.(*super.TypeRecord); ok {
			fields[i].Val = r.fields[i].Val.(*Record).ChangeType(rtyp)
		}
	}
	return &Record{typ, fields, r.len}
}

func (r *Record) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	if r.Typ.Opts != 0 {
		// XXX TBD: improve performance of this in summit
		var nones []int
		var optOff int
		for k := range r.fields {
			fslot := int32(slot)
			if r.Typ.Fields[k].Opt {
				if slotmap := r.fields[k].slotmap(); len(slotmap) > 0 {
					fslot = slotmap[slot]
				}
				if fslot < 0 {
					nones = append(nones, optOff)
					optOff++
					continue
				}
				optOff++
			}
			r.fields[k].Val.Serialize(b, uint32(fslot))
		}
		b.EndContainerWithNones(r.Typ.Opts, nones)
		return
	}
	for k := range r.fields {
		r.fields[k].Val.Serialize(b, slot)
	}
	b.EndContainer()
}

type Field struct {
	Val Any
	Len uint32
	// Runs encode the run-lengths of alternating nones and values.
	// Runs is always non-empty for an optional field since all nones is a single value
	// and all values is a single 0 followed by number of values.  This means we can
	// test len(Run) for optional or not.
	Runs  []uint32
	mu    sync.Mutex
	slots []int32 // map record slot to field slot (-1 for none) (from Nones or builder)
	dyn   Any
}

func (f *Field) Deref(sctx *super.Context) Any {
	if len(f.Runs) == 0 {
		return f.Val
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dyn == nil {
		if f.Val.Len() == 0 {
			// No values... all nones.
			return NewMissing(sctx, f.Len)
		}
		tags, noneLen := buildTags(f.Runs, f.Len)
		if noneLen == 0 {
			// This field is optional but everything is here in this instance.
			f.dyn = f.Val
		}
		errs := NewMissing(sctx, f.Len-noneLen)
		f.dyn = NewDynamic(tags, []Any{f.Val, errs})
	}
	return f.dyn
}

func (f *Field) slotmap() []int32 {
	if len(f.Runs) == 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.slots == nil {
		f.slots = make([]int32, f.Len)
		var noneLen uint32
		var off uint32
		var valOff uint32
		for in := 0; in < len(f.Runs); {
			noneRun := f.Runs[in]
			in++
			for k := range noneRun {
				f.slots[off+k] = -1
			}
			off += noneRun
			noneLen += noneRun
			if in >= len(f.Runs) {
				break
			}
			// skip over values (leaving bits 0)
			valRun := f.Runs[in]
			for k := range valRun {
				f.slots[off+k] = int32(valOff + uint32(k))
			}
			valOff += valRun
			off += valRun
			in++
		}
	}
	return f.slots
}

func buildTags(nones []uint32, n uint32) ([]uint32, uint32) {
	tags := make([]uint32, n)
	off := 0
	var noneLen uint32
	for in := 0; in < len(nones); {
		noneRun := nones[in]
		in++
		for k := range int(noneRun) {
			tags[off+k] = 1
		}
		off += int(noneRun)
		noneLen += noneRun
		if in >= len(nones) {
			break
		}
		// skip over values (leaving tags 0)
		off += int(nones[in])
		in++
	}
	return tags, noneLen
}

// RLE emits a sequence of runs of the length of alternating sequences
// of nones and values, beginning with nones.  Every run is non-zero except for
// the first, which may be zero when the first value is non-none.
type RLE struct {
	runs       []uint32
	running    bool
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

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

// XXX this will be cleaned up in a subsequent PR when we add a proper vector.Option
func NewOptionFromRLE(sctx *super.Context, vec Any, length uint32, runlens []uint32) Any {
	typ := vec.Type()
	if super.IsOptionType(typ) {
		panic(typ)
	}
	optionType := sctx.Option(typ)
	if union, ok := vec.(*Union); ok {
		// If it's a union, let's make it an option type by adding type none at the end
		types := slices.Clone(union.Typ.Types)
		types = append(types, super.TypeNone)
		vecs := slices.Clone(union.Dynamic.Values)
		noneTag := uint32(len(vecs))
		// buildTags assumes a single value at tag 0 and a none at tag 1.
		// we'll build that then convert it from this unions tags, where the
		// union tags are preserved and the none at tag 1 goes to the last tag (noneTag).
		tags, noneLen := buildTags(runlens, length)
		vecs = append(vecs, NewNone(noneLen))
		from := 0
		for k := range tags {
			if tags[k] == 0 {
				tags[k] = union.Tags[from]
				from++
			} else {
				tags[k] = noneTag
			}
		}
		return NewUnion(optionType, tags, vecs)
	}
	tags, noneLen := buildTags(runlens, length)
	vecs := []Any{vec, NewNone(noneLen)}
	return NewUnion(optionType, tags, vecs)
}

// XXX this will be reworked in a subsequent PR
// option values should have two forms like typevalues...
// the union form with a type none in the union and the RLE form,
// with the underlying compact vector... we turn RLEs into tags
// on demand so that when we load record with lots of optional fields
// (as is common with fusion) then we only unroll the tags on demand
// and an optional field can round trip from vcache to runtime and back
// to CSUP without the tags ever being built.
type Optional struct {
	typ *super.TypeUnion
	*Dynamic
}

func (o *Optional) Type() super.Type {
	return o.typ
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

// XXX this will be cleaned up in a subsequent PR
func deoptionApply(sctx *super.Context, vec Any) Any {
	switch vec := vec.(type) {
	case *Optional:
		return vec.Dynamic
	case *None:
		return NewMissing(sctx, vec.len)
	}
	return vec
}

func DeoptionWithMissing(sctx *super.Context, vec Any) Any {
	switch vec := vec.(type) {
	case *None:
		return NewMissing(sctx, vec.Len())
	case *Dynamic:
		if hasOptionTypesOrNones(vec.Values) {
			vecs := make([]Any, 0, len(vec.Values))
			for _, v := range vec.Values {
				vecs = append(vecs, DeoptionWithMissing(sctx, v))
			}
			return stitch(vec.Tags, vecs)
		}
	case *Union:
		if super.IsOptionType(vec.Typ) {
			out := Deunion(vec)
			out = DeoptionWithMissing(sctx, out)
			return out
		}
	case *Optional:
		return vec.Dynamic
	}
	return vec
}

func hasOptionTypesOrNones(vecs []Any) bool {
	return slices.IndexFunc(vecs, func(vec Any) bool {
		// XXX apparently the runtime sometimes creates nil vectors inside
		// of Dynamics where said vector is never referenced by a tag
		// (e.g., at the output of vector switch), so we check for nil here.
		if vec == nil {
			return false
		}
		if vec, ok := vec.(*Dynamic); ok {
			return hasOptionTypesOrNones(vec.Values)
		}
		return super.IsOptionType(vec.Type()) || vec.Type() == super.TypeNone
	}) >= 0
}

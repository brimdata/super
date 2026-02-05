package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Record struct {
	Typ    *super.TypeRecord
	Fields []Any
	len    uint32
}

// XXX We need field to be a vector because
type Field struct {
	Values Any
	Nones  []int32 // run lengths to build dynamic XXX we can make this better
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

func (r *Record) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	if r.Typ.Opts != 0 {
		// XXX this is a hack to detect a dynamic with val/missing and presume
		// this should be turned into a none... error(missing) should be error(missing)
		// and we should have a non-transparent way to represent the None condition
		// in the runtime, i.e., you can't do anything with None except for operators
		// that assert noneness one way or another and no assertion turns into error(missing).
		// XXX we will change this in summit
		var nones []int
		var optOff int
		for k, f := range r.Fields {
			if r.Typ.Fields[k].Opt {
				if d := isOpt(f); d != nil {
					if d.Tags[slot] == 1 {
						nones = append(nones, optOff)
						optOff++
						continue
					}
				}
				optOff++
			}
			f.Serialize(b, slot)
		}
		b.EndContainerWithNones(r.Typ.Opts, nones)
		return
	}
	for _, f := range r.Fields {
		f.Serialize(b, slot)
	}
	b.EndContainer()
}

func isOpt(v Any) *Dynamic {
	if d, ok := v.(*Dynamic); ok && len(d.Values) == 2 && d.Values[1].Kind() == KindError {
		return d
	}
	return nil
}

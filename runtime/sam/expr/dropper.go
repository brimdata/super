package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/scode"
)

type Dropper struct {
	sctx      *super.Context
	dropMap   fieldsMap
	dropType  map[super.Type]super.Type
	emptyType super.Type
	builder   scode.Builder
}

func NewDropper(sctx *super.Context, fields field.List) *Dropper {
	dropMap := fieldsMap{}
	for _, f := range fields {
		dropMap.Add(f)
	}
	return &Dropper{
		sctx:      sctx,
		dropMap:   dropMap,
		dropType:  make(map[super.Type]super.Type),
		emptyType: sctx.MustLookupTypeRecord(nil),
	}
}

func (d *Dropper) recode(b *scode.Builder, typ super.Type, bytes scode.Bytes, dropMap fieldsMap) {
	recType := super.TypeRecordOf(typ)
	if recType == nil {
		b.Append(bytes)
		return
	}
	b.BeginContainer()
	it := scode.NewRecordIter(bytes, recType.Opts)
	var optOff int
	var nones []int
	for _, f := range recType.Fields {
		elem, none := it.Next(f.Opt)
		dropMapChild, ok := dropMap[f.Name]
		if ok {
			if dropMapChild == nil {
				continue
			}
			if none {
				nones = append(nones, optOff)
				optOff++
				continue
			}
			d.recode(b, f.Type, elem, dropMapChild)
		} else if none {
			nones = append(nones, optOff)
		} else {
			b.Append(elem)
		}
		if f.Opt {
			optOff++
		}
	}
	b.EndContainerWithNones(recType.Opts, nones)
}

func (d *Dropper) Eval(in super.Value) super.Value {
	typ := in.Type()
	dropType, ok := d.dropType[typ]
	if !ok {
		dropType = d.dropMap.dropType(d.sctx, typ)
		d.dropType[typ] = dropType
	}
	if dropType == typ {
		return in
	}
	if dropType == d.emptyType {
		return d.sctx.Quiet()
	}
	b := &d.builder
	b.Reset()
	d.recode(b, typ, in.Bytes(), d.dropMap)
	return super.NewValue(dropType, b.Bytes().Body())
}

type fieldsMap map[string]fieldsMap

func (f fieldsMap) Add(path field.Path) {
	if len(path) == 1 {
		f[path[0]] = nil
	} else if len(path) > 1 {
		ff, ok := f[path[0]]
		if ff == nil {
			if ok {
				return
			}
			ff = fieldsMap{}
			f[path[0]] = ff
		}
		ff.Add(path[1:])
	}
}

func (f fieldsMap) dropType(sctx *super.Context, typ super.Type) super.Type {
	if named, ok := typ.(*super.TypeNamed); ok {
		inner := f.dropType(sctx, named.Type)
		if inner == named.Type {
			return typ
		}
		return inner
	}
	recType := super.TypeRecordOf(typ)
	if recType == nil {
		return typ
	}
	var out []super.Field
	for _, field := range recType.Fields {
		typ := field.Type
		ff, ok := f[field.Name]
		if ok {
			if ff == nil {
				continue
			}
			typ = ff.dropType(sctx, typ)
		}
		out = append(out, super.NewFieldWithOpt(field.Name, typ, field.Opt))
	}
	return sctx.MustLookupTypeRecord(out)
}

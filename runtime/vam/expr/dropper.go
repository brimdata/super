package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

// Dropper drops one or more fields in a record.  If it drops all fields of a
// top-level record, the record is replaced by error("quiet").  If it drops all
// fields of a nested record, the nested record is dropped.  Dropper does not
// modify non-records.
type Dropper struct {
	zctx *super.Context
	fm   fieldsMap
}

func NewDropper(zctx *super.Context, fields field.List) *Dropper {
	fm := fieldsMap{}
	for _, f := range fields {
		fm.Add(f)
	}
	return &Dropper{zctx, fm}
}

func (d *Dropper) Eval(vec vector.Any) vector.Any {
	return vector.Apply(false, d.eval, vec)
}

func (d *Dropper) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if vec.Type().Kind() != super.RecordKind {
		return vec
	}
	if vec2, ok := d.drop(vec, d.fm); ok {
		if vec2 == nil {
			// Dropped all fields.
			return vector.NewStringError(d.zctx, "quiet", vec.Len())
		}
		return vec2
	}
	return vec
}

// drop drops the fields in fm from vec.  It returns nil, false if vec is not a
// record or no fields were dropped; nil, true if all fields were dropped; and
// non-nil, true if some fields were dropped or modified.
func (d *Dropper) drop(vec vector.Any, fm fieldsMap) (vector.Any, bool) {
	switch vec := vector.Under(vec).(type) {
	case *vector.Record:
		fields := super.TypeRecordOf(vec.Type()).Fields
		var changed bool
		var newFields []super.Field
		var newVecs []vector.Any
		for i, f := range fields {
			if ff, ok := fm[f.Name]; ok {
				if ff == nil {
					// Drop field.
					if !changed {
						newFields = slices.Clone(fields[:i])
						newVecs = slices.Clone(vec.Fields[:i])
						changed = true
					}
					continue
				}
				if vec2, ok := d.drop(vec.Fields[i], ff); ok {
					// Field changed.
					if !changed {
						newFields = slices.Clone(fields[:i])
						newVecs = slices.Clone(vec.Fields[:i])
						changed = true
					}
					if vec2 == nil {
						// Drop field since we dropped all its subfields.
						continue
					}
					// Substitute modified field.
					newFields = append(newFields, super.NewField(f.Name, vec2.Type()))
					newVecs = append(newVecs, vec2)
					continue
				}
			}
			// Keep field.
			if changed {
				newFields = append(newFields, f)
				newVecs = append(newVecs, vec.Fields[i])
			}
		}
		if !changed {
			return nil, false
		}
		if len(newFields) == 0 {
			return nil, true
		}
		newRecType := d.zctx.MustLookupTypeRecord(newFields)
		return vector.NewRecord(newRecType, newVecs, vec.Len(), vec.Nulls), true
	case *vector.Dict:
		if newVec, ok := d.drop(vec.Any, fm); ok {
			return vector.NewDict(newVec, vec.Index, vec.Counts, vec.Nulls), true
		}
	case *vector.View:
		if newVec, ok := d.drop(vec.Any, fm); ok {
			return vector.NewView(vec.Index, newVec), true
		}
	}
	return vec, false
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

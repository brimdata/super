package function

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#unflatten
type Unflatten struct {
	zctx *super.Context

	builder     zcode.Builder
	recordCache recordCache

	// These exist only to reduce memory allocations.
	path   field.Path
	types  []super.Type
	values []zcode.Bytes
}

func NewUnflatten(zctx *super.Context) *Unflatten {
	return &Unflatten{
		zctx: zctx,
	}
}

func (u *Unflatten) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0]
	array, ok := super.TypeUnder(val.Type()).(*super.TypeArray)
	if !ok {
		return val
	}
	u.recordCache.reset()
	root := u.recordCache.new()
	u.types = u.types[:0]
	u.values = u.values[:0]
	for it := val.Bytes().Iter(); !it.Done(); {
		bytes := it.Next()
		path, typ, vb, err := u.parseElem(array.Type, bytes)
		if err != nil {
			return u.zctx.WrapError(err.Error(), super.NewValue(array.Type, bytes))
		}
		if typ == nil {
			continue
		}
		if removed := root.addPath(&u.recordCache, path); removed > 0 {
			u.types = u.types[:len(u.types)-removed]
			u.values = u.values[:len(u.values)-removed]
		}
		u.types = append(u.types, typ)
		u.values = append(u.values, vb)
	}
	u.builder.Reset()
	types, values := u.types, u.values
	typ, err := root.build(u.zctx, &u.builder, func() (super.Type, zcode.Bytes) {
		typ, value := types[0], values[0]
		types, values = types[1:], values[1:]
		return typ, value
	})
	if err != nil {
		return u.zctx.WrapError(err.Error(), val)
	}
	return super.NewValue(typ, u.builder.Bytes())
}

func (u *Unflatten) parseElem(inner super.Type, vb zcode.Bytes) (field.Path, super.Type, zcode.Bytes, error) {
	if union, ok := super.TypeUnder(inner).(*super.TypeUnion); ok {
		inner, vb = union.Untag(vb)
	}
	typ := super.TypeRecordOf(inner)
	if typ == nil || len(typ.Fields) != 2 {
		return nil, nil, nil, nil
	}
	nkey, ok := typ.IndexOfField("key")
	if !ok {
		return nil, nil, nil, nil
	}

	vtyp, ok := typ.TypeOfField("value")
	if !ok {
		return nil, nil, nil, nil
	}
	it := vb.Iter()
	kbytes, vbytes := it.Next(), it.Next()
	if nkey == 1 {
		kbytes, vbytes = vbytes, kbytes
	}
	ktyp := typ.Fields[nkey].Type
	if ktyp.ID() == super.IDString {
		u.path = append(u.path[:0], super.DecodeString(kbytes))
		return u.path, vtyp, vbytes, nil
	}
	if a, ok := super.TypeUnder(ktyp).(*super.TypeArray); ok && a.Type.ID() == super.IDString {
		return u.decodeKey(kbytes), vtyp, vbytes, nil
	}
	return nil, nil, nil, fmt.Errorf("invalid key type %s: expected either string or [string]", sup.FormatType(ktyp))
}

func (u *Unflatten) decodeKey(b zcode.Bytes) field.Path {
	u.path = u.path[:0]
	for it := b.Iter(); !it.Done(); {
		u.path = append(u.path, super.DecodeString(it.Next()))
	}
	return u.path
}

type recordCache struct {
	index   int
	records []*record
}

func (c *recordCache) new() *record {
	if c.index == len(c.records) {
		c.records = append(c.records, new(record))
	}
	r := c.records[c.index]
	r.fields = r.fields[:0]
	r.records = r.records[:0]
	c.index++
	return r
}

func (c *recordCache) reset() {
	c.index = 0
}

type record struct {
	fields  []super.Field
	records []*record
}

func (r *record) addPath(c *recordCache, p []string) (removed int) {
	if len(p) == 0 {
		return 0
	}
	at := len(r.fields) - 1
	if len(r.fields) == 0 || r.fields[at].Name != p[0] {
		r.fields = append(r.fields, super.NewField(p[0], nil))
		var rec *record
		if len(p) > 1 {
			rec = c.new()
		}
		r.records = append(r.records, rec)
	} else if len(p) == 1 || r.records[at] == nil {
		// If this isn't a new field and we're either at a leaf or the
		// previously value was a leaf, we're stacking on a previously created
		// record and need to signal that values have been removed.
		removed = r.records[at].countLeaves()
		if len(p) > 1 {
			r.records[at] = c.new()
		} else {
			r.records[at] = nil
		}
	}
	return removed + r.records[len(r.records)-1].addPath(c, p[1:])
}

func (r *record) countLeaves() int {
	if r == nil {
		return 1
	}
	var count int
	for _, rec := range r.records {
		count += rec.countLeaves()
	}
	return count
}

func (r *record) build(zctx *super.Context, b *zcode.Builder, next func() (super.Type, zcode.Bytes)) (super.Type, error) {
	for i, rec := range r.records {
		if rec == nil {
			typ, value := next()
			b.Append(value)
			r.fields[i].Type = typ
			continue
		}
		b.BeginContainer()
		var err error
		r.fields[i].Type, err = rec.build(zctx, b, next)
		if err != nil {
			return nil, err
		}
		b.EndContainer()
	}
	return zctx.LookupTypeRecord(r.fields)
}

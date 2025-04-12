package vcache

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type union struct {
	mu   sync.Mutex
	meta *csup.Union
	count
	// XXX we should store TagMap here so it doesn't have to be recomputed
	tags   []uint32
	values []shadow
	nulls  *nulls
}

func newUnion(cctx *csup.Context, meta *csup.Union, nulls *nulls) *union {
	return &union{
		meta:   meta,
		values: make([]shadow, len(meta.Values)),
		nulls:  nulls,
		count:  count{meta.Len(cctx), nulls.count()},
	}
}

func (u *union) unmarshal(cctx *csup.Context, projection field.Projection) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for k, id := range u.meta.Values {
		if u.values[k] == nil {
			u.values[k] = newShadow(cctx, id, nil)
		}
		u.values[k].unmarshal(cctx, projection)
	}
}

func (u *union) load(loader *loader) ([]uint32, bitvec.Bits) {
	nulls := u.nulls.get(loader)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.tags != nil {
		return u.tags, nulls
	}
	tags, err := csup.ReadUint32s(u.meta.Tags, loader.r)
	if err != nil {
		panic(err)
	}
	u.tags = tags
	return tags, nulls
}

func (u *union) project(loader *loader, projection field.Projection) vector.Any {
	vecs := make([]vector.Any, 0, len(u.values))
	types := make([]super.Type, 0, len(u.values))
	for _, shadow := range u.values {
		vec := shadow.project(loader, projection)
		vecs = append(vecs, vec)
		types = append(types, vec.Type())
	}
	utyp := loader.sctx.LookupTypeUnion(types)
	tags, nulls := u.load(loader)
	// If there are nulls add a null vector and rebuild tags.
	if !nulls.IsZero() {
		var newtags []uint32
		n := uint32(len(vecs))
		var nullcount uint32
		for i := range nulls.Len() {
			if nulls.IsSet(i) {
				newtags = append(newtags, n)
				nullcount++
			} else {
				newtags = append(newtags, tags[0])
				tags = tags[1:]
			}
		}
		tags = newtags
		vecs = append(vecs, vector.NewConst(super.NewValue(utyp, nil), nullcount, bitvec.Zero))
	}
	return vector.NewUnion(utyp, tags, vecs, nulls)
}

type unionLoader struct {
	loader *loader
	shadow *union
}

var _ vector.Uint32Loader = (*unionLoader)(nil)

func (u *unionLoader) Load() ([]uint32, bitvec.Bits) {
	return u.shadow.load(u.loader)
}

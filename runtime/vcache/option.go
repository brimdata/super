package vcache

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type option struct {
	mu   sync.Mutex
	meta *csup.Option
	len  uint32
	// XXX we should store TagMap here so it doesn't have to be recomputed
	tags   []uint32
	values shadow
}

func newOption(cctx *csup.Context, meta *csup.Option) *option {
	return &option{
		meta: meta,
		len:  meta.Len(cctx),
	}
}

func (o *option) length() uint32 {
	return o.len
}

func (o *option) unmarshal(cctx *csup.Context, projection field.Projection) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.values == nil {
		o.values = newShadow(cctx, o.meta.Values)
	}
	o.values.unmarshal(cctx, projection)
}

func (o *option) load(loader *loader) []uint32 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.tags != nil {
		return o.tags
	}
	tags, err := csup.ReadUint32s(o.meta.Tags, loader.r)
	if err != nil {
		panic(err)
	}
	o.tags = tags
	return tags
}

func (o *option) project(loader *loader, projection field.Projection) vector.Any {
	sctx := loader.sctx
	typ, err := sctx.TranslateType(o.meta.Type)
	if err != nil {
		panic(err)
	}
	optionType := typ.(*super.TypeOption)
	vec := o.values.project(loader, projection)
	if vec.Kind() == vector.KindNone {
		return vector.NewOption(optionType, vec)
	}
	nones := o.len - vec.Len()
	if nones == 0 {
		return vector.NewOption(optionType, vec)
	}
	tags := o.load(loader)
	return vector.NewOption(optionType, vector.NewDynamic(tags, []vector.Any{vector.NewNone(nones), vec}))
}

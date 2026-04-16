package op

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/vam/expr/function"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Fuse struct {
	sctx     *super.Context
	parent   vio.Puller
	complete bool

	fuser    *samagg.Fuser
	vecs     []vector.Any
	upcaster *function.Upcast
}

func NewFuse(sctx *super.Context, parent vio.Puller, complete bool) *Fuse {
	return &Fuse{
		sctx:     sctx,
		parent:   parent,
		complete: complete,
		upcaster: function.NewUpcast(sctx),
	}
}

func (f *Fuse) Pull(done bool) (vector.Any, error) {
	if done {
		f.fuser = nil
		f.vecs = nil
		return f.parent.Pull(done)
	}
	if f.fuser == nil {
		f.fuser = samagg.NewFuser(f.sctx, f.complete)
		for {
			vec, err := f.parent.Pull(false)
			if err != nil {
				return nil, err
			}
			if vec == nil {
				break
			}
			if d, ok := vec.(*vector.Dynamic); ok {
				for _, vec := range d.Values {
					if vec != nil {
						f.fuser.Fuse(vec.Type())
					}
				}
			} else {
				f.fuser.Fuse(vec.Type())
			}
			f.vecs = append(f.vecs, vec)
		}
	}
	if len(f.vecs) == 0 {
		f.fuser = nil
		f.vecs = nil
		return nil, nil
	}
	vec := f.vecs[0]
	f.vecs[0] = nil
	f.vecs = f.vecs[1:]
	return vector.Apply(false, f.upcast, vec), nil
}

func (f *Fuse) upcast(vecs ...vector.Any) vector.Any {
	typ := f.fuser.Type()
	out, ok := f.upcaster.Cast(vecs[0], typ)
	if !ok {
		return vector.NewWrappedError(f.sctx, "cannot upcast to "+sup.FormatType(typ), vecs[0])
	}
	return out
}

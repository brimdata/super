package vector

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Fusion struct {
	Sctx   *super.Context
	Typ    *super.TypeFusion
	Values Any
	// We materialize the the subtypes of the supertype
	// only when needed to build a Dynamic from the fused type.
	// This is typically not required for many operations, in which
	// case the type IDs are never read from storage and the types
	// are never built and entered into the context.
	mu       sync.Mutex
	loader   TypeLoader
	subtypes []super.Type
}

type TypeLoader interface {
	Load() []super.Type
}

var _ Any = (*Union)(nil)

func NewFusion(sctx *super.Context, typ *super.TypeFusion, vals Any, subtypes []super.Type) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, Values: vals, subtypes: subtypes}
}

func NewFusionWithLoader(sctx *super.Context, typ *super.TypeFusion, loader TypeLoader, vals Any) *Fusion {
	return &Fusion{Sctx: sctx, Typ: typ, loader: loader, Values: vals}
}

func (*Fusion) Kind() Kind {
	return KindFusion
}

func (f *Fusion) Type() super.Type {
	return f.Typ
}

func (f *Fusion) Len() uint32 {
	return f.Values.Len()
}

func (f *Fusion) Serialize(b *scode.Builder, slot uint32) {
	b.BeginContainer()
	f.Values.Serialize(b, slot)
	// XXX this is a slow path
	typeVal := f.Sctx.LookupTypeValue(f.Subtypes()[slot])
	b.Append(typeVal.Bytes())
	b.EndContainer()
}

func (f *Fusion) Subtypes() []super.Type {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subtypes == nil {
		f.subtypes = f.loader.Load()
	}
	return f.subtypes
}

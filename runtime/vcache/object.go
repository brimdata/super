package vcache

import (
	"context"

	"github.com/superdb/super"
	"github.com/superdb/super/bsup"
	"github.com/superdb/super/pkg/field"
	"github.com/superdb/super/pkg/storage"
	"github.com/superdb/super/vector"
)

// Object is the interface to load a given BSUP object from storage into
// memory and perform projections (or whole value reads) of the in-memory data.
// This is also suitable for one-pass use where the data is read on demand,
// used for processing, then discarded.  Objects maybe be persisted across
// multiple callers of Cache and the super.Context in use is passed in for
// each vector constructed from its in-memory shadow.
type Object struct {
	object *bsup.Object
	root   shadow
}

// NewObject creates a new in-memory Object corresponding to a BSUP object
// residing in storage.  The BSUP header and metadata section are read and
// the metadata is deserialized so that vectors can be loaded into the cache
// on demand only as needed and retained in memory for future use.
func NewObject(ctx context.Context, engine storage.Engine, uri *storage.URI) (*Object, error) {
	// XXX currently we open a storage.Reader for every object and never close it.
	// We should either close after a timeout and reopen when needed or change the
	// storage API to have a more reasonable semantics around the Put/Get not leaving
	// a file descriptor open for every long Get.  Perhaps there should be another
	// method for intermittent random access.
	// XXX maybe open the reader inside Fetch if needed?
	reader, err := engine.Get(ctx, uri)
	if err != nil {
		return nil, err
	}
	object, err := bsup.NewObject(reader)
	if err != nil {
		return nil, err
	}
	return NewObjectFromBSUP(object), nil
}

func NewObjectFromBSUP(object *bsup.Object) *Object {
	return &Object{object: object}
}

func (o *Object) Close() error {
	return o.object.Close()
}

// Fetch returns the indicated projection of data in this BSUP object.
// If any required data is not memory resident, it will be fetched from
// storage and cached in memory so that subsequent calls run from memory.
// The vectors returned will have types from the provided sctx.  Multiple
// Fetch calls to the same object may run concurrently.
func (o *Object) Fetch(sctx *super.Context, projection field.Projection) (vector.Any, error) {
	cctx := o.object.Context()
	loader := &loader{cctx, sctx, o.object.DataReader()}
	o.root = newShadow(cctx, o.object.Root())
	o.root.unmarshal(cctx, projection)
	return loader.load(projection, o.root)
}

// FetchUnordered is like Fetch, but if o's root vector is dynamic,
// FetchUnordered returns the underlying values vectors instead of a
// vector.Dynamic.
func (o *Object) FetchUnordered(vecs []vector.Any, sctx *super.Context, projection field.Projection) ([]vector.Any, error) {
	cctx := o.object.Context()
	o.root = newShadow(cctx, o.object.Root())
	o.root.unmarshal(cctx, projection)
	loader := &loader{cctx: cctx, sctx: sctx, r: o.object.DataReader()}
	if d, ok := o.root.(*dynamic); ok {
		return d.projectUnordered(vecs, loader, projection), nil
	}
	vec, err := loader.load(projection, o.root)
	if err != nil {
		return nil, err
	}
	return append(vecs, vec), nil
}

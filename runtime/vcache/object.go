package vcache

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vng"
)

// Object is the interface to load a given VNG object from storage into
// memory and perform projections (or whole value reads) of the in-memory data.
// This is also suitable for one-pass use where the data is read on demand,
// used for processing, then discarded.  Objects maybe be persisted across
// multiple callers of Cache and the super.Context in use is passed in for
// each vector constructed from its in-memory shadow.
type Object struct {
	object *vng.Object
	root   shadow
}

// NewObject creates a new in-memory Object corresponding to a VNG object
// residing in storage.  The VNG header and metadata section are read and
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
	object, err := vng.NewObject(reader)
	if err != nil {
		return nil, err
	}
	return NewObjectFromVNG(object), nil
}

func NewObjectFromVNG(object *vng.Object) *Object {
	return &Object{
		object: object,
		root:   newShadow(object.Metadata(), nil, 0),
	}
}

func (o *Object) Close() error {
	return o.object.Close()
}

// Fetch returns the indicated projection of data in this VNG object.
// If any required data is not memory resident, it will be fetched from
// storage and cached in memory so that subsequent calls run from memory.
// The vectors returned will have types from the provided zctx.  Multiple
// Fetch calls to the same object may run concurrently.
func (o *Object) Fetch(zctx *super.Context, projection Path) (vector.Any, error) {
	return (&loader{zctx, o.object.DataReader()}).load(projection, o.root)
}

package vcache

import (
	"fmt"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

// The shadow type mirrors the vector.Any implementations here with locks and
// pointers to shared vector slices.  This lets us page in just the portions
// of vector data that is needed at any given time (which we cache inside the shadow).
// When we need a runtime vector, we build the immutable vector.Any components from
// mutable shadow pieces that are dynamically loaded and maintained here.
//
// Shadows are created incrementally so that a sequence of projections will do the
// minimal work unmarshaling the CSUP metadata as needed.  When processing a sequence
// of CSUP files with a single projection, the incremental capability is not important
// but when caching CSUP objects (e.g., in local from S3), multiple threads operating
// concurrently on a single object benefit from incremental unmarshaling.  This is especially
// important when processing thin projections over objects with lots of heteregenous types.
//
// Note that the shadow doesn't know about the query type context, thereby allowing the shadow
// to be shared across different queries.  Instead, the loader that builds a vector.Any
// is reponsible for computing the shared type from the shadow hierarchy.
//
// Shadows are created with unmarshal and only the portion of the shadow tree is
// created for the passed-in projection.  Any shadow object may be updated
// incrementally and concurrently by calling the unmarshal method to unfurl additional
// legs of a value (e.g., via subsequent calls with different projections).
// The project method creates a vector.Any from the umarshaled shadow, which
// should cover the passed in projection.
//
// The shadow locking also supports incremental expansion and loading from the
// vector runtime via the *Loader implementations of the various vector loader
// interfaces.
type shadow interface {
	length() uint32
	unmarshal(*csup.Context, field.Projection)
	project(*loader, field.Projection) vector.Any
}

type count struct {
	vals  uint32
	nulls uint32
}

func (c count) length() uint32 {
	return c.nulls + c.vals
}

// newShadow decodes the CSUP metadata structure to the appropriate shadow object.
// It also unfurls null metadata and links together parent pointers so that nulls
// can be properly flattened but only as needed on demand.
// No vector data or null data is actually loaded here.
func newShadow(cctx *csup.Context, id csup.ID, nulls *nulls) shadow {
	switch meta := cctx.Lookup(id).(type) {
	case *csup.Dynamic:
		return newDynamic(meta)
	case *csup.Nulls:
		return newShadow(cctx, meta.Values, newNulls(meta, nulls))
	case *csup.Error:
		return newError(cctx, meta, nulls)
	case *csup.Named:
		return newNamed(meta, newShadow(cctx, meta.Values, nulls))
	case *csup.Record:
		return newRecord(cctx, meta, nulls)
	case *csup.Array:
		return newArray(cctx, meta, nulls)
	case *csup.Set:
		return newSet(cctx, meta, nulls)
	case *csup.Map:
		return newMap(cctx, meta, nulls)
	case *csup.Union:
		return newUnion(cctx, meta, nulls)
	case *csup.Dict:
		return newDict(cctx, meta, nulls)
	case *csup.Int:
		return newInt(cctx, meta, nulls)
	case *csup.Uint:
		return newUint(cctx, meta, nulls)
	case *csup.Float:
		return newFloat(cctx, meta, nulls)
	case *csup.Bytes:
		return newBytes(cctx, meta, nulls)
	case *csup.Primitive:
		return newPrimitive(cctx, meta, nulls)
	case *csup.Const:
		return newConst(cctx, meta, nulls)
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", meta))
	}
}

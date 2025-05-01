package csup

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"golang.org/x/sync/errgroup"
)

type Encoder interface {
	// Write collects up values to be encoded into memory.
	Write(zcode.Bytes)
	// Encode encodes all in-memory vector data into its storage-ready serialized format.
	// Vectors may be encoded concurrently and errgroup.Group is used to sync
	// and return errors.
	Encode(*errgroup.Group)
	// Metadata returns the data structure conforming to the CSUP specification
	// describing the layout of vectors.  This is called after all data is
	// written and encoded by the Encode with the result marshaled to build
	// the header section of the CSUP object.  An offset is passed down into
	// the traversal representing where in the data section the vector data
	// will land.  This is called in a sequential fashion (no parallelism) so
	// that the metadata can be computed and the CSUP header written before the
	// vector data is written via Emit.
	Metadata(*Context, uint64) (uint64, ID)
	Emit(w io.Writer) error
}

type PrimitiveEncoder interface {
	Encoder
	Dict() (PrimitiveEncoder, []byte, []uint32)
	ConstValue() super.Value
}

func NewEncoder(typ super.Type) Encoder {
	switch typ := typ.(type) {
	case *super.TypeNamed:
		return &NamedEncoder{NewEncoder(typ.Type), typ.Name}
	case *super.TypeError:
		return &ErrorEncoder{NewEncoder(typ.Type)}
	case *super.TypeRecord:
		return NewNullsEncoder(NewRecordEncoder(typ))
	case *super.TypeArray:
		return NewNullsEncoder(NewArrayEncoder(typ))
	case *super.TypeSet:
		// Sets encode the same way as arrays but behave
		// differently semantically, and we don't care here.
		return NewNullsEncoder(NewSetEncoder(typ))
	case *super.TypeMap:
		return NewNullsEncoder(NewMapEncoder(typ))
	case *super.TypeUnion:
		return NewNullsEncoder(NewUnionEncoder(typ))
	case *super.TypeEnum:
		return NewNullsEncoder(NewPrimitiveEncoder(typ))
	default:
		if !super.IsPrimitiveType(typ) {
			panic(fmt.Sprintf("unsupported type in CSUP file: %T", typ))
		}
		return NewNullsEncoder(NewDictEncoder(typ, NewPrimitiveEncoder(typ)))
	}
}

func NewPrimitiveEncoder(typ super.Type) PrimitiveEncoder {
	switch id := typ.ID(); {
	case super.IsSigned(id):
		return NewIntEncoder(typ)
	case super.IsUnsigned(id):
		return NewUintEncoder(typ)
	case super.IsFloat(id):
		return NewFloatEncoder(typ)
	case id == super.IDBytes || id == super.IDString || id == super.IDType:
		return NewBytesEncoder(typ)
	default:
		return NewZcodeEncoder(typ)
	}
}

type NamedEncoder struct {
	Encoder
	name string
}

func (n *NamedEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, id := n.Encoder.Metadata(cctx, off)
	return off, cctx.enter(&Named{n.name, id})
}

type ErrorEncoder struct {
	Encoder
}

func (e *ErrorEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, id := e.Encoder.Metadata(cctx, off)
	return off, cctx.enter(&Error{id})
}

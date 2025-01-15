package vng

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
	// Metadata returns the data structure conforming to the VNG specification
	// describing the layout of vectors.  This is called after all data is
	// written and encoded by the Encode with the result marshaled to build
	// the header section of the VNG object.  An offset is passed down into
	// the traversal representing where in the data section the vector data
	// will land.  This is called in a sequential fashion (no parallelism) so
	// that the metadata can be computed and the VNG header written before the
	// vector data is written via Emit.
	Metadata(uint64) (uint64, Metadata)
	Emit(w io.Writer) error
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
			panic(fmt.Sprintf("unsupported type in VNG file: %T", typ))
		}
		var enc Encoder
		switch id := typ.ID(); {
		case super.IsSigned(id):
			enc = NewIntEncoder(typ)
		case super.IsUnsigned(id):
			enc = NewUintEncoder(typ)
		default:
			enc = NewPrimitiveEncoder(typ)
		}
		return NewNullsEncoder(NewDictEncoder(typ, enc))
	}
}

type resetter interface {
	reset()
}

type NamedEncoder struct {
	Encoder
	name string
}

func (n *NamedEncoder) Metadata(off uint64) (uint64, Metadata) {
	off, meta := n.Encoder.Metadata(off)
	return off, &Named{n.name, meta}
}

type ErrorEncoder struct {
	Encoder
}

func (e *ErrorEncoder) Metadata(off uint64) (uint64, Metadata) {
	off, meta := e.Encoder.Metadata(off)
	return off, &Error{meta}
}

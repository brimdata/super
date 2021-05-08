package zst

import (
	"errors"
	"io"

	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zst/column"
)

var ErrBadSchemaID = errors.New("bad schema id in root reassembly column")

type Assembly struct {
	root    zng.Value
	schemas []*zng.TypeRecord
	columns []*zng.Record
}

func NewAssembler(a *Assembly, seeker *storage.Seeker) (*Assembler, error) {
	assembler := &Assembler{
		root:    &column.Int{},
		schemas: a.schemas,
	}
	if err := assembler.root.UnmarshalZNG(a.root, seeker); err != nil {
		return nil, err
	}
	assembler.columns = make([]*column.Record, len(a.schemas))
	for k := 0; k < len(a.schemas); k++ {
		rec := a.columns[k]
		zv := rec.Value
		record_col := &column.Record{}
		if err := record_col.UnmarshalZNG(a.schemas[k], zv, seeker); err != nil {
			return nil, err
		}
		assembler.columns[k] = record_col
	}
	return assembler, nil
}

// Assembler implements the zbuf.Reader and io.Closer.  It reads a columnar
// zst object to generate a stream of zng.Records.  It also has methods
// to read metainformation for test and debugging.
type Assembler struct {
	root    *column.Int
	columns []*column.Record
	schemas []*zng.TypeRecord
	builder zcode.Builder
	err     error
}

func (a *Assembler) Read() (*zng.Record, error) {
	a.builder.Reset()
	schemaID, err := a.root.Read()
	if err == io.EOF {
		return nil, nil
	}
	if schemaID < 0 || int(schemaID) >= len(a.columns) {
		return nil, ErrBadSchemaID
	}
	col := a.columns[schemaID]
	if col == nil {
		return nil, ErrBadSchemaID
	}
	err = col.Read(&a.builder)
	if err != nil {
		return nil, err
	}
	body, err := a.builder.Bytes().ContainerBody()
	if err != nil {
		return nil, err
	}
	rec := zng.NewRecord(a.schemas[schemaID], body)
	//XXX if we had a buffer pool where records could be built back to
	// back in batches, then we could get rid of this extra allocation
	// and copy on every record
	rec.Keep()
	return rec, nil
}

package parquetio

import (
	"errors"
	"io"

	"github.com/brimdata/zed"
	goparquet "github.com/fraugster/parquet-go"
)

type Reader struct {
	fr  *goparquet.FileReader
	typ *zed.TypeRecord

	builder builder
	val     zed.Value
}

func NewReader(zctx *zed.Context, r io.Reader) (*Reader, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		return nil, errors.New("reader cannot seek")
	}
	fr, err := goparquet.NewFileReader(rs)
	if err != nil {
		return nil, err
	}
	typ, err := newRecordType(zctx, fr.GetSchemaDefinition().RootColumn.Children)
	if err != nil {
		return nil, err
	}
	return &Reader{
		fr:  fr,
		typ: typ,
	}, nil
}

func (r *Reader) Read() (*zed.Value, error) {
	data, err := r.fr.NextRow()
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	r.builder.Truncate()
	for _, f := range r.typ.Fields {
		r.builder.appendValue(f.Type, data[f.Name])
	}
	r.val = *zed.NewValue(r.typ, r.builder.Bytes())
	return &r.val, nil
}

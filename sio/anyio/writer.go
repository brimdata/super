package anyio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/runtime/vam/expr/function"
	"github.com/brimdata/super/sio/arrowio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/csvio"
	"github.com/brimdata/super/sio/dbio"
	"github.com/brimdata/super/sio/jsonio"
	"github.com/brimdata/super/sio/lineio"
	"github.com/brimdata/super/sio/parquetio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sio/tableio"
	"github.com/brimdata/super/sio/zeekio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type WriterOpts struct {
	Format    string
	SUPFusion bool
	BSUP      *bsupio.WriterOpts // Nil means use defaults via bsupio.NewWriter.
	CSV       csvio.WriterOpts
	DB        dbio.WriterOpts
	JSON      jsonio.WriterOpts
	SUP       supio.WriterOpts
}

func NewWriter(w io.WriteCloser, opts WriterOpts) (vio.PushCloser, error) {
	switch opts.Format {
	case "arrows":
		return newDefuser(arrowio.NewWriter(w)), nil
	case "bsup":
		if opts.BSUP == nil {
			return bsupio.NewWriter(w), nil
		}
		return bsupio.NewWriterWithOpts(w, *opts.BSUP), nil
	case "csup":
		return csup.NewSerializer(w), nil
	case "csv":
		return newDefuser(csvio.NewWriter(w, opts.CSV)), nil
	case "db":
		return newDefuser(dbio.NewWriter(w, opts.DB)), nil
	case "json":
		return newDefuser(jsonio.NewWriter(w, opts.JSON)), nil
	case "line":
		return newDefuser(lineio.NewWriter(w)), nil
	case "null":
		return &nullWriter{}, nil
	case "parquet":
		return newDefuser(parquetio.NewWriter(w)), nil
	case "sup", "":
		w := vio.PushCloser(supio.NewWriter(w, opts.SUP))
		if !opts.SUPFusion {
			w = newDefuser(w)
		}
		return w, nil
	case "table":
		return newDefuser(tableio.NewWriter(w)), nil
	case "tsv":
		opts.CSV.Delim = '\t'
		return newDefuser(csvio.NewWriter(w, opts.CSV)), nil
	case "zeek":
		return newDefuser(zeekio.NewWriter(w)), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", opts.Format)
	}
}

type defuser struct {
	vio.PushCloser
	defuse expr.Function
}

func newDefuser(w vio.PushCloser) vio.PushCloser {
	return &defuser{PushCloser: w, defuse: function.NewDefuse(super.NewContext())}
}

func (d *defuser) Push(vec vector.Any) error {
	label, ok := vec.(*vector.Labeled)
	if ok {
		vec = label.Any
	}
	if vec != nil {
		vec = vector.Apply(vector.ApplyNone, d.defuse.Call, vec)
	}
	if ok {
		vec = &vector.Labeled{Any: vec, Label: label.Label}
	}
	return d.PushCloser.Push(vec)
}

type nullWriter struct{}

func (*nullWriter) Push(vector.Any) error {
	return nil
}

func (*nullWriter) Close() error {
	return nil
}

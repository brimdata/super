package supio

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type Writer struct {
	writer    io.WriteCloser
	formatter *sup.StreamFormatter
	defuse    expr.Function
}

type WriterOpts struct {
	ColorDisabled bool
	Fusion        bool
	Pretty        int
}

func NewWriter(w io.WriteCloser, opts WriterOpts) *Writer {
	var defuse expr.Function
	if !opts.Fusion {
		defuse = function.NewDefuse(super.NewContext())
	}
	return &Writer{
		formatter: sup.NewStreamFormatter(opts.Pretty, opts.ColorDisabled),
		writer:    w,
		defuse:    defuse,
	}
}

func (w *Writer) Push(vec vector.Any) error {
	return sbuf.WriteVec(w, vec)
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(val super.Value) error {
	if w.defuse != nil {
		val = w.defuse.Call([]super.Value{val})
	}
	if _, err := io.WriteString(w.writer, w.formatter.FormatValue(val)); err != nil {
		return err
	}
	_, err := w.writer.Write([]byte("\n"))
	return err
}

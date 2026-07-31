package supio

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/tsup"
)

type Reader struct {
	reader   io.Reader
	sctx     *super.Context
	parser   *tsup.Parser
	analyzer *tsup.Analyzer
	builder  *scode.Builder
	val      super.Value
}

func NewReader(sctx *super.Context, r io.Reader) *Reader {
	return &Reader{
		reader:   r,
		sctx:     sctx,
		analyzer: tsup.NewAnalyzer(sctx),
		builder:  scode.NewBuilder(),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	if r.parser == nil {
		r.parser = tsup.NewParser(r.reader)
	}
	ast, err := r.parser.ParseValue()
	if ast == nil || err != nil {
		return nil, err
	}
	val, err := r.analyzer.ConvertValue(ast)
	if err != nil {
		return nil, err
	}
	r.val, err = tsup.Build(r.builder, val)
	return &r.val, err
}

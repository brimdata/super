package supio

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

type Reader struct {
	reader   io.Reader
	sctx     *super.Context
	parser   *sup.Parser
	analyzer sup.Analyzer
	builder  *zcode.Builder
	val      super.Value
}

func NewReader(sctx *super.Context, r io.Reader) *Reader {
	return &Reader{
		reader:   r,
		sctx:     sctx,
		analyzer: sup.NewAnalyzer(),
		builder:  zcode.NewBuilder(),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	if r.parser == nil {
		r.parser = sup.NewParser(r.reader)
	}
	ast, err := r.parser.ParseValue()
	if ast == nil || err != nil {
		return nil, err
	}
	val, err := r.analyzer.ConvertValue(r.sctx, ast)
	if err != nil {
		return nil, err
	}
	r.val, err = sup.Build(r.builder, val)
	return &r.val, err
}

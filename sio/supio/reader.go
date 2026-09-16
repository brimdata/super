package supio

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type Reader struct {
	reader   io.Reader
	sctx     *super.Context
	parser   *sup.Parser
	analyzer *sup.Analyzer
	builder  *scode.Builder
	val      super.Value
}

func NewReader(sctx *super.Context, r io.Reader) *Reader {
	return &Reader{
		reader:   r,
		sctx:     sctx,
		analyzer: sup.NewAnalyzer(sctx),
		builder:  scode.NewBuilder(),
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
	val, err := r.analyzer.ConvertValue(ast)
	if err != nil {
		return nil, err
	}
	r.val, err = sup.Build(r.builder, val)
	return &r.val, err
}

func (r *Reader) Type() (super.Type, error) {
	r.parser = sup.NewParser(r.reader)
	fuser := super.NewFuser(r.sctx, false)
	for {
		ast, err := r.parser.ParseValue()
		if ast == nil || err != nil {
			return fuser.Type(), err
		}
		val, err := r.analyzer.ConvertValue(ast)
		if err != nil {
			return nil, err
		}
		fuser.Fuse(val.Type())
	}
}

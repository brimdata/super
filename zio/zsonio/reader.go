package zsonio

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

type Reader struct {
	reader   io.Reader
	zctx     *super.Context
	parser   *zson.Parser
	analyzer zson.Analyzer
	builder  *zcode.Builder
	val      super.Value
}

func NewReader(zctx *super.Context, r io.Reader) *Reader {
	return &Reader{
		reader:   r,
		zctx:     zctx,
		analyzer: zson.NewAnalyzer(),
		builder:  zcode.NewBuilder(),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	if r.parser == nil {
		r.parser = zson.NewParser(r.reader)
	}
	ast, err := r.parser.ParseValue()
	if ast == nil || err != nil {
		return nil, err
	}
	val, err := r.analyzer.ConvertValue(r.zctx, ast)
	if err != nil {
		return nil, err
	}
	r.val, err = zson.Build(r.builder, val)
	return &r.val, err
}

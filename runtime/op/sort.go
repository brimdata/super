package op

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Sort struct {
	rctx    *runtime.Context
	samsort *sort.Op
}

func NewSort(rctx *runtime.Context, parent vio.Puller, fields []expr.SortExpr, guessReverse bool) *Sort {
	materializer := sbuf.NewMaterializer(parent)
	s := sort.New(rctx, materializer, fields, guessReverse)
	return &Sort{rctx: rctx, samsort: s}
}

func (s *Sort) Pull(done bool) (vector.Any, error) {
	batch, err := s.samsort.Pull(done)
	if batch == nil || err != nil {
		return nil, err
	}
	b := vector.NewDynamicValueBuilder()
	for _, val := range batch.Values() {
		b.Write(val)
	}
	return b.Build(s.rctx.Sctx), nil
}

package op

import (
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/vector"
)

type Sort struct {
	rctx    *runtime.Context
	samsort *sort.Op
}

func NewSort(rctx *runtime.Context, parent vector.Puller, fields []expr.SortExpr, nullsFirst, guessReverse bool, resetter expr.Resetter) *Sort {
	materializer := vam.NewMaterializer(parent)
	s := sort.New(rctx, materializer, fields, nullsFirst, guessReverse, resetter)
	return &Sort{rctx: rctx, samsort: s}
}

func (s *Sort) Pull(done bool) (vector.Any, error) {
	batch, err := s.samsort.Pull(done)
	if batch == nil || err != nil {
		return nil, err
	}
	b := vector.NewDynamicBuilder()
	for _, val := range batch.Values() {
		b.Write(val)
	}
	return b.Build(), nil
}

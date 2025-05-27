package vcache

import (
	"io"
	"iter"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zio/bsupio"
)

type bsup struct {
	meta *csup.BSUP
}

func newBSUP(_ *csup.Context, meta *csup.BSUP, _ *nulls) *bsup {
	return &bsup{meta}
}

func (b *bsup) length() uint32 {
	return b.meta.Count
}

func (*bsup) unmarshal(*csup.Context, field.Projection) {}

func (b *bsup) project(loader *loader, _ field.Projection) vector.Any {
	var vb vector.Builder
	for i := range b.meta.Count {
		val, err, ok := loader.nextBSUPValue()
		if !ok {
			panic("not enough BSUP values")
		}
		if err != nil {
			panic(err)
		}
		if i == 0 {
			vb = vector.NewBuilder(val.Type())
		}
		vb.Write(val.Bytes())
	}
	return vb.Build(bitvec.Zero)
}

func readBSUPAndProjectAndTranslateType(sctx *super.Context, r io.Reader, projection field.Projection) iter.Seq2[*super.Value, error] {
	return func(yield func(*super.Value, error) bool) {
		bsupCtx := super.NewContext()
		br := bsupio.NewReader(sctx, r)
		defer br.Close()
		projExpr := newProjectionExpr(bsupCtx, projection)
		translated := map[super.Type]super.Type{}
		for {
			val, err := br.Read()
			if err != nil {
				yield(val, err)
				return
			}
			if val == nil {
				return
			}
			projVal := projExpr.Eval(nil, *val)
			typ, ok := translated[projVal.Type()]
			if !ok {
				typ, err = sctx.TranslateType(projVal.Type())
				if err != nil {
					yield(nil, err)
					return
				}
			}
			translated[projVal.Type()] = typ
			translatedVal := super.NewValue(typ, projVal.Bytes())
			if !yield(&translatedVal, nil) {
				return
			}
		}
	}
}

func newProjectionExpr(sctx *super.Context, projection field.Projection) expr.Evaluator {
	if len(projection) == 0 {
		return &expr.This{}
	}
	var elems []expr.RecordElem
	for _, node := range projection {
		e := expr.NewDottedExpr(sctx, field.Path{node.Name})
		if len(node.Proj) > 0 {
			e = &chainedExpr{e, newProjectionExpr(sctx, node.Proj)}
		}
		elems = append(elems, expr.RecordElem{Name: node.Name, Field: e})
	}
	e, err := expr.NewRecordExpr(sctx, elems)
	if err != nil {
		panic(err)
	}
	return e
}

type chainedExpr struct {
	first  expr.Evaluator
	second expr.Evaluator
}

func (c *chainedExpr) Eval(ectx expr.Context, this super.Value) super.Value {
	val := c.first.Eval(ectx, this)
	if val.IsMissing() {
		return val
	}
	return c.second.Eval(ectx, val)
}

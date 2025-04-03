package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// Index represents an index operator "container[index]" where container is
// either an array or set (with index type integer), or a record
// (with index type string), or a map (with any index type).
type Index struct {
	zctx      *super.Context
	container Evaluator
	index     Evaluator
}

func NewIndexExpr(zctx *super.Context, container, index Evaluator) Evaluator {
	return &Index{zctx, container, index}
}

func (i *Index) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, i.eval, this)
}

func (i *Index) eval(args ...vector.Any) vector.Any {
	this := args[0]
	container := i.container.Eval(this)
	index := i.index.Eval(this)
	switch vector.KindOf(vector.Under(container)) {
	case vector.KindArray, vector.KindSet:
		return indexArrayOrSet(i.zctx, container, index)
	case vector.KindRecord:
		return indexRecord(i.zctx, container, index)
	case vector.KindMap:
		panic("vector index operations on maps not supported")
	default:
		return vector.NewMissing(i.zctx, this.Len())
	}
}

func indexArrayOrSet(zctx *super.Context, vec, indexVec vector.Any) vector.Any {
	if !super.IsInteger(indexVec.Type().ID()) {
		return vector.NewWrappedError(zctx, "index is not an integer", indexVec)
	}
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec, index = view.Any, view.Index
	}
	offsets, vals, nulls := arrayOrSetContents(vec)
	indexVec = promoteToSigned(indexVec)
	var errs []uint32
	var viewIndexes []uint32
	for i := range indexVec.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		idxVal, isnull := vector.IntValue(indexVec, uint32(i))
		if !nulls.Value(idx) && !isnull && idxVal != 0 {
			start := offsets[idx]
			len := int64(offsets[idx+1]) - int64(start)
			if idxVal < 0 {
				idxVal = len + idxVal
			} else {
				idxVal--
			}
			if idxVal >= 0 && idxVal < len {
				viewIndexes = append(viewIndexes, start+uint32(idxVal))
				continue
			}
		}
		errs = append(errs, i)
	}
	out := vector.Deunion(vector.NewView(vals, viewIndexes))
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewMissing(zctx, uint32(len(errs))))
	}
	return out
}

func indexRecord(zctx *super.Context, vec, indexVec vector.Any) vector.Any {
	if indexVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(zctx, "record index is not a string", indexVec)
	}
	var rec *vector.Record
	var index []uint32
	switch vec := vec.(type) {
	case *vector.Record:
		rec = vec
	case *vector.View:
		rec, index = vec.Any.(*vector.Record), vec.Index
	default:
		panic(vec)
	}
	var errcnt uint32
	tags := make([]uint32, vec.Len())
	n := len(rec.Typ.Fields)
	viewIndexes := make([][]uint32, n)
	for i := range vec.Len() {
		field, _ := vector.StringValue(indexVec, i)
		k, ok := rec.Typ.IndexOfField(field)
		if !ok {
			tags[i] = uint32(n)
			errcnt++
			continue
		}
		idx := i
		if index != nil {
			idx = index[i]
		}
		tags[i] = uint32(k)
		viewIndexes[k] = append(viewIndexes[k], idx)
	}
	out := make([]vector.Any, n+1)
	out[n] = vector.NewMissing(zctx, errcnt)
	for i, field := range rec.Fields {
		out[i] = vector.NewView(field, viewIndexes[i])
	}
	return vector.NewDynamic(tags, out)
}

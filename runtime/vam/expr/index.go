package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
)

// Index represents an index operator "container[index]" where container is
// either an array or set (with index type integer), or a record
// (with index type string), or a map (with any index type).
type Index struct {
	sctx      *super.Context
	container Evaluator
	index     Evaluator
	base1     bool
}

func NewIndexExpr(sctx *super.Context, container, index Evaluator, base1 bool) Evaluator {
	return &Index{sctx, container, index, base1}
}

func (i *Index) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, i.eval, this)
}

func (i *Index) eval(args ...vector.Any) vector.Any {
	this := args[0]
	container := vector.Under(i.container.Eval(this))
	index := vector.Under(i.index.Eval(this))
	switch vector.KindOf(container) {
	case vector.KindArray, vector.KindSet:
		return indexArrayOrSet(i.sctx, container, index, i.base1)
	case vector.KindRecord:
		return indexRecord(i.sctx, container, index, i.base1)
	case vector.KindMap:
		return indexMap(i.sctx, container, index)
	default:
		return vector.NewMissing(i.sctx, this.Len())
	}
}

func indexArrayOrSet(sctx *super.Context, vec, indexVec vector.Any, base1 bool) vector.Any {
	if _, ok := indexVec.(*vector.Error); ok {
		return indexVec
	}
	if id := indexVec.Type().ID(); super.IsUnsigned(id) {
		return vector.Apply(true, func(args ...vector.Any) vector.Any {
			return indexArrayOrSet(sctx, args[0], args[1], base1)
		}, vec, cast.To(sctx, indexVec, super.TypeInt64))
	} else if !super.IsInteger(id) {
		return vector.NewWrappedError(sctx, "index is not an integer", indexVec)
	}
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec, index = view.Any, view.Index
	}
	offsets, vals, nulls := arrayOrSetContents(vec)
	var errs []uint32
	var viewIndexes []uint32
	for i := range indexVec.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		idxVal, isnull := vector.IntValue(indexVec, uint32(i))
		if !nulls.IsSet(idx) && !isnull {
			start := offsets[idx]
			len := int64(offsets[idx+1]) - int64(start)
			if idxVal < 0 {
				idxVal = len + idxVal
			} else if base1 {
				idxVal--
			}
			if idxVal >= 0 && idxVal < len {
				viewIndexes = append(viewIndexes, start+uint32(idxVal))
				continue
			}
		}
		errs = append(errs, i)
	}
	out := vector.Deunion(vector.Pick(vals, viewIndexes))
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewMissing(sctx, uint32(len(errs))))
	}
	return out
}

func indexRecord(sctx *super.Context, vec, indexVec vector.Any, base1 bool) vector.Any {
	var isint bool
	switch id := indexVec.Type().ID(); {
	case super.IsUnsigned(id):
		return vector.Apply(true, func(args ...vector.Any) vector.Any {
			return indexRecord(sctx, args[0], args[1], base1)
		}, vec, cast.To(sctx, indexVec, super.TypeInt64))
	case super.IsSigned(id):
		isint = true
	case id == super.IDString:
	default:
		if indexVec.Type().Kind() == super.ErrorKind {
			return indexVec
		}
		return vector.NewWrappedError(sctx, "invalid value for record index", indexVec)
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
		var k int
		if isint {
			idx, _ := vector.IntValue(indexVec, i)
			k = int(idx)
			if k < 0 {
				k = n + k
			} else if base1 {
				k--
			}
		} else {
			field, _ := vector.StringValue(indexVec, i)
			var ok bool
			if k, ok = rec.Typ.IndexOfField(field); !ok {
				k = -1
			}
		}
		if k < 0 || k >= n {
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
	out[n] = vector.NewMissing(sctx, errcnt)
	for i, field := range rec.Fields {
		out[i] = vector.Pick(field, viewIndexes[i])
	}
	return vector.NewDynamic(tags, out)
}

func indexMap(sctx *super.Context, vec, indexVec vector.Any) vector.Any {
	// XXX At some point we'll want to optimize having a const indexVec.
	if _, ok := indexVec.(*vector.Error); ok {
		return indexVec
	}
	n := vec.Len()
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	m := vec.(*vector.Map)
	var pick []uint32
	for idx := range n {
		i := idx
		if index != nil {
			i = index[idx]
		}
		for range m.Offsets[i+1] - m.Offsets[i] {
			pick = append(pick, idx)
		}
	}
	var valIndexes, errs []uint32
	cmp := NewCompare(sctx, "==", nil, nil).eval
	hits := vector.Apply(true, cmp, vector.Pick(indexVec, pick), m.Keys)
	bits := toBool(hits).Bits
	for idx := range n {
		i := idx
		if index != nil {
			i = index[idx]
		}
		selected := -1
		for off := m.Offsets[i]; off < m.Offsets[i+1]; off++ {
			if bits.IsSet(off) {
				selected = int(off)
				break
			}
		}
		if selected != -1 {
			valIndexes = append(valIndexes, uint32(selected))
		} else {
			errs = append(errs, idx)
		}
	}
	vals := vector.Pick(vector.Deunion(m.Values), valIndexes)
	if len(errs) > 0 {
		return vector.Combine(vals, errs, vector.NewMissing(sctx, uint32(len(errs))))
	}
	return vals
}

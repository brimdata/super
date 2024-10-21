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
	switch val := vector.Under(container).(type) {
	case *vector.Array:
		return indexArrayOrSet(i.zctx, val.Offsets, val.Values, index, val.Nulls)
	case *vector.Set:
		return indexArrayOrSet(i.zctx, val.Offsets, val.Values, index, val.Nulls)
	case *vector.Record:
		return indexRecord(i.zctx, val, index)
	case *vector.Map:
		panic("vector index operations on maps not supported")
	default:
		return vector.NewMissing(i.zctx, this.Len())
	}
}

func indexArrayOrSet(zctx *super.Context, offsets []uint32, vals, index vector.Any, nulls *vector.Bool) vector.Any {
	if !super.IsInteger(index.Type().ID()) {
		return vector.NewWrappedError(zctx, "index is not an integer", index)
	}
	index = promoteToSigned(index)
	var errs []uint32
	var viewIndexes []uint32
	for i, start := range offsets[:len(offsets)-1] {
		idx, idxNull := vector.IntValue(index, uint32(i))
		if !nulls.Value(uint32(i)) && !idxNull {
			len := int64(offsets[i+1]) - int64(start)
			if idx < 0 {
				idx = len + idx
			}
			if idx >= 0 && idx < len {
				viewIndexes = append(viewIndexes, start+uint32(idx))
				continue
			}
		}
		errs = append(errs, uint32(i))
	}
	out := vector.Deunion(vector.NewView(viewIndexes, vals))
	if len(errs) > 0 {
		return vector.Combine(out, errs, vector.NewMissing(zctx, uint32(len(errs))))
	}
	return out
}

func indexRecord(zctx *super.Context, record *vector.Record, index vector.Any) vector.Any {
	if index.Type().ID() != super.IDString {
		return vector.NewWrappedError(zctx, "record index is not a string", index)
	}
	var errcnt uint32
	tags := make([]uint32, record.Len())
	n := len(record.Typ.Fields)
	viewIndexes := make([][]uint32, n)
	for i := uint32(0); i < record.Len(); i++ {
		field, _ := vector.StringValue(index, i)
		k, ok := record.Typ.IndexOfField(field)
		if !ok {
			tags[i] = uint32(n)
			errcnt++
			continue
		}
		tags[i] = uint32(k)
		viewIndexes[k] = append(viewIndexes[k], i)
	}
	out := make([]vector.Any, n+1)
	out[n] = vector.NewMissing(zctx, errcnt)
	for i, field := range record.Fields {
		out[i] = vector.NewView(viewIndexes[i], field)
	}
	return vector.NewDynamic(tags, out)
}

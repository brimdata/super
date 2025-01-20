package expr

import (
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
)

type sliceExpr struct {
	zctx                            *super.Context
	containerEval, fromEval, toEval Evaluator
}

func NewSliceExpr(zctx *super.Context, container, from, to Evaluator) Evaluator {
	return &sliceExpr{
		zctx:          zctx,
		containerEval: container,
		fromEval:      from,
		toEval:        to,
	}
}

func (s *sliceExpr) Eval(vec vector.Any) vector.Any {
	vecs := []vector.Any{s.containerEval.Eval(vec)}
	if s.fromEval != nil {
		vecs = append(vecs, s.fromEval.Eval(vec))
	}
	if s.toEval != nil {
		vecs = append(vecs, s.toEval.Eval(vec))
	}
	return vector.Apply(true, s.eval, vecs...)
}

func (s *sliceExpr) eval(vecs ...vector.Any) vector.Any {
	container := vecs[0]
	var from, to vector.Any
	vecs = vecs[1:]
	if s.fromEval != nil {
		from = vecs[0]
		if !super.IsSigned(from.Type().ID()) {
			return vector.NewStringError(s.zctx, "slice index is not a number", from.Len())
		}
		vecs = vecs[1:]
	}
	if s.toEval != nil {
		to = vecs[0]
		if !super.IsSigned(to.Type().ID()) {
			return vector.NewStringError(s.zctx, "slice index is not a number", to.Len())
		}
	}
	switch vector.KindOf(container) {
	case vector.KindArray, vector.KindSet:
		return s.evalArrayOrSlice(container, from, to)
	case vector.KindBytes, vector.KindString:
		return s.evalStringOrBytes(container, from, to)
	case vector.KindError:
		return container
	default:
		return vector.NewWrappedError(s.zctx, "sliced value is not array, set, bytes, or string", container)
	}
}

func (s *sliceExpr) evalArrayOrSlice(vec, fromVec, toVec vector.Any) vector.Any {
	from, constFrom := sliceIsConstIndex(fromVec)
	to, constTo := sliceIsConstIndex(toVec)
	slowPath := !constFrom || !constTo
	var index []uint32
	n := vec.Len()
	if view, ok := vec.(*vector.View); ok {
		vec, index = view.Any, view.Index
	}
	offsets, inner, nullsIn := arrayOrSetContents(vec)
	newOffsets := []uint32{0}
	var errs []uint32
	var innerIndex []uint32
	var nullsOut *vector.Bool
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if nullsIn.Value(idx) {
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1])
			if nullsOut == nil {
				nullsOut = vector.NewBoolEmpty(vec.Len(), nil)
			}
			nullsOut.Set(i)
			continue
		}
		off := offsets[idx]
		size := int64(offsets[idx+1] - off)
		start, end := int64(0), size
		if fromVec != nil {
			if slowPath {
				from, _ = vector.IntValue(fromVec, i)
			}
			start = sliceIndex(from, size)
		}
		if toVec != nil {
			if slowPath {
				to, _ = vector.IntValue(toVec, i)
			}
			end = sliceIndex(to, size)
		}
		if invalidSlice(start, end, size) {
			errs = append(errs, i)
			continue
		}
		newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(end-start))
		for k := start; k < end; k++ {
			innerIndex = append(innerIndex, off+uint32(k))
		}

	}
	var out vector.Any
	inner = vector.NewView(inner, innerIndex)
	if vector.KindOf(vec) == vector.KindArray {
		out = vector.NewArray(vec.Type().(*super.TypeArray), newOffsets, inner, nullsOut)
	} else {
		out = vector.NewSet(vec.Type().(*super.TypeSet), newOffsets, inner, nullsOut)
	}
	if nullsOut != nil {
		nullsOut.SetLen(out.Len())
	}
	if len(errs) > 0 {
		errOut := vector.NewStringError(s.zctx, "slice out of bounds", uint32(len(errs)))
		return vector.Combine(out, errs, errOut)
	}
	return out
}

func (s *sliceExpr) evalStringOrBytes(vec, fromVec, toVec vector.Any) vector.Any {
	constFrom, isConstFrom := sliceIsConstIndex(fromVec)
	constTo, isConstTo := sliceIsConstIndex(toVec)
	if isConstFrom && isConstTo {
		if out, ok := s.evalStringOrBytesFast(vec, constFrom, constTo); ok {
			return out
		}
	}
	var errs []uint32
	newOffsets := []uint32{0}
	var newBytes []byte
	var nullsOut *vector.Bool
	id := vec.Type().ID()
	for i := range vec.Len() {
		slice, isnull := s.bytesAt(vec, i)
		if isnull {
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1])
			if nullsOut == nil {
				nullsOut = vector.NewBoolEmpty(vec.Len(), nil)
			}
			nullsOut.Set(i)
			continue
		}
		size := lengthOfBytesOrString(id, slice)
		start, end := int64(0), size
		if fromVec != nil {
			from, _ := vector.IntValue(fromVec, i)
			start = sliceIndex(from, size)
		}
		if toVec != nil {
			to, _ := vector.IntValue(toVec, i)
			end = sliceIndex(to, size)
		}
		if invalidSlice(start, end, size) {
			errs = append(errs, i)
			continue
		}
		slice = sliceBytesOrString(slice, id, start, end)
		newBytes = append(newBytes, slice...)
		newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(len(slice)))

	}
	out := s.bytesOrStringVec(vec.Type(), newOffsets, newBytes, nullsOut)
	if nullsOut != nil {
		nullsOut.SetLen(out.Len())
	}
	if len(errs) > 0 {
		errOut := vector.NewStringError(s.zctx, "slice out of bounds", uint32(len(errs)))
		return vector.Combine(out, errs, errOut)
	}
	return out
}

func (s *sliceExpr) evalStringOrBytesFast(vec vector.Any, from, to int64) (vector.Any, bool) {
	switch vec := vec.(type) {
	case *vector.Const:
		slice := vec.Value().Bytes()
		id := vec.Type().ID()
		size := lengthOfBytesOrString(id, slice)
		start, end := int64(0), size
		if s.fromEval != nil {
			start = sliceIndex(from, size)
		}
		if s.toEval != nil {
			end = sliceIndex(to, size)
		}
		if invalidSlice(start, end, size) {
			return nil, false
		}
		slice = sliceBytesOrString(slice, id, start, end)
		return vector.NewConst(super.NewValue(vec.Type(), slice), vec.Len(), vec.Nulls), true
	case *vector.View:
		out, ok := s.evalStringOrBytesFast(vec.Any, from, to)
		if !ok {
			return nil, false
		}
		return vector.NewView(out, vec.Index), true
	case *vector.Dict:
		out, ok := s.evalStringOrBytesFast(vec.Any, from, to)
		if !ok {
			return nil, false
		}
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls), true
	default:
		offsets, bytes, nullsIn := stringOrBytesContents(vec)
		newOffsets := []uint32{0}
		var newBytes []byte
		id := vec.Type().ID()
		for i := range vec.Len() {
			slice := bytes[offsets[i]:offsets[i+1]]
			size := lengthOfBytesOrString(id, slice)
			start, end := int64(0), size
			if s.fromEval != nil {
				start = sliceIndex(from, size)
			}
			if s.toEval != nil {
				end = sliceIndex(to, size)
			}
			if invalidSlice(start, end, size) {
				return nil, false
			}
			slice = sliceBytesOrString(slice, id, start, end)
			newBytes = append(newBytes, slice...)
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(len(slice)))
		}
		return s.bytesOrStringVec(vec.Type(), newOffsets, newBytes, nullsIn), true
	}
}

func (s *sliceExpr) bytesOrStringVec(typ super.Type, offsets []uint32, bytes []byte, nulls *vector.Bool) vector.Any {
	switch typ.ID() {
	case super.IDBytes:
		return vector.NewBytes(offsets, bytes, nulls)
	case super.IDString:
		return vector.NewString(offsets, bytes, nulls)
	default:
		panic(typ)
	}
}

func (s *sliceExpr) bytesAt(val vector.Any, slot uint32) ([]byte, bool) {
	switch val := val.(type) {
	case *vector.String:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		return val.Bytes[val.Offsets[slot]:val.Offsets[slot+1]], false
	case *vector.Bytes:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		return val.Value(slot), false
	case *vector.Const:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		s, _ := val.AsBytes()
		return s, false
	case *vector.Dict:
		if val.Nulls.Value(slot) {
			return nil, true
		}
		return s.bytesAt(val.Any, uint32(val.Index[slot]))
	case *vector.View:
		return s.bytesAt(val.Any, val.Index[slot])
	}
	panic(val)
}

func lengthOfBytesOrString(id int, slice []byte) int64 {
	if id == super.IDString {
		return int64(utf8.RuneCount(slice))
	}
	return int64(len(slice))
}

func invalidSlice(start, end, size int64) bool {
	return start > end || end > size || start < 0
}

func sliceIsConstIndex(vec vector.Any) (int64, bool) {
	if vec == nil {
		return 0, true
	}
	if c, ok := vec.(*vector.Const); ok && c.Nulls == nil {
		return c.Value().Int(), true
	}
	return 0, false
}

func sliceIndex(idx, size int64) int64 {
	if idx < 0 {
		idx += int64(size)
	}
	return idx
}

func sliceBytesOrString(slice []byte, id int, start, end int64) []byte {
	if id == super.IDString {
		slice = slice[expr.UTF8PrefixLen(slice, int(start)):]
		return slice[:expr.UTF8PrefixLen(slice, int(end-start))]
	} else {
		return slice[start:end]
	}
}

func stringOrBytesContents(vec vector.Any) ([]uint32, []byte, *vector.Bool) {
	switch vec := vec.(type) {
	case *vector.String:
		return vec.Offsets, vec.Bytes, vec.Nulls
	case *vector.Bytes:
		return vec.Offs, vec.Bytes, vec.Nulls
	default:
		panic(vec)
	}
}

func arrayOrSetContents(vec vector.Any) ([]uint32, vector.Any, *vector.Bool) {
	switch vec := vec.(type) {
	case *vector.Array:
		return vec.Offsets, vec.Values, vec.Nulls
	case *vector.Set:
		return vec.Offsets, vec.Values, vec.Nulls
	default:
		panic(vec)
	}
}

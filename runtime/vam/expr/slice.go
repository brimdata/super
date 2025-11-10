package expr

import (
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type sliceExpr struct {
	sctx                            *super.Context
	containerEval, fromEval, toEval Evaluator
	base1                           bool
}

func NewSliceExpr(sctx *super.Context, container, from, to Evaluator, base1 bool) Evaluator {
	return &sliceExpr{
		sctx:          sctx,
		containerEval: container,
		fromEval:      from,
		toEval:        to,
		base1:         base1,
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
			return vector.NewStringError(s.sctx, "slice index is not a number", from.Len())
		}
		vecs = vecs[1:]
	}
	if s.toEval != nil {
		to = vecs[0]
		if !super.IsSigned(to.Type().ID()) {
			return vector.NewStringError(s.sctx, "slice index is not a number", to.Len())
		}
	}
	switch vector.KindOf(container) {
	case vector.KindArray, vector.KindSet:
		return s.evalArrayOrSlice(container, from, to, s.base1)
	case vector.KindBytes, vector.KindString:
		return s.evalStringOrBytes(container, from, to, s.base1)
	case vector.KindError:
		return container
	default:
		return vector.NewWrappedError(s.sctx, "sliced value is not array, set, bytes, or string", container)
	}
}

func (s *sliceExpr) evalArrayOrSlice(vec, fromVec, toVec vector.Any, base1 bool) vector.Any {
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
	var innerIndex []uint32
	var nullsOut bitvec.Bits
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		if nullsIn.IsSet(idx) {
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1])
			if nullsOut.IsZero() {
				nullsOut = bitvec.NewFalse(vec.Len())
			}
			nullsOut.Set(i)
			continue
		}
		off := offsets[idx]
		size := int(offsets[idx+1] - off)
		start, end := 0, size
		if fromVec != nil {
			if slowPath {
				v, _ := vector.IntValue(fromVec, i)
				from = int(v)
			}
			start = sliceIndex(from, size, base1)
		}
		if toVec != nil {
			if slowPath {
				v, _ := vector.IntValue(toVec, i)
				to = int(v)
			}
			end = sliceIndex(to, size, base1)
		}
		start, end = expr.FixSliceBounds(start, end, size)
		newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(end-start))
		for k := start; k < end; k++ {
			innerIndex = append(innerIndex, off+uint32(k))
		}
	}
	var out vector.Any
	inner = vector.Pick(inner, innerIndex)
	if vector.KindOf(vec) == vector.KindArray {
		out = vector.NewArray(vec.Type().(*super.TypeArray), newOffsets, inner, nullsOut)
	} else {
		out = vector.NewSet(vec.Type().(*super.TypeSet), newOffsets, inner, nullsOut)
	}
	if !nullsOut.IsZero() {
		nullsOut.Shorten(out.Len())
	}
	return out
}

func (s *sliceExpr) evalStringOrBytes(vec, fromVec, toVec vector.Any, base1 bool) vector.Any {
	constFrom, isConstFrom := sliceIsConstIndex(fromVec)
	constTo, isConstTo := sliceIsConstIndex(toVec)
	if isConstFrom && isConstTo {
		if out, ok := s.evalStringOrBytesFast(vec, constFrom, constTo, base1); ok {
			return out
		}
	}
	newOffsets := []uint32{0}
	var newBytes []byte
	var nullsOut bitvec.Bits
	id := vec.Type().ID()
	for i := range vec.Len() {
		slice, isnull := s.bytesAt(vec, i)
		if isnull {
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1])
			if !nullsOut.IsZero() {
				nullsOut = bitvec.NewFalse(vec.Len())
			}
			nullsOut.Set(i)
			continue
		}
		size := lengthOfBytesOrString(id, slice)
		start, end := 0, size
		if fromVec != nil {
			from, _ := vector.IntValue(fromVec, i)
			start = sliceIndex(int(from), size, base1)
		}
		if toVec != nil {
			to, _ := vector.IntValue(toVec, i)
			end = sliceIndex(int(to), size, base1)
		}
		start, end = expr.FixSliceBounds(start, end, size)
		slice = sliceBytesOrString(slice, id, start, end)
		newBytes = append(newBytes, slice...)
		newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(len(slice)))

	}
	out := s.bytesOrStringVec(vec.Type(), newOffsets, newBytes, nullsOut)
	if !nullsOut.IsZero() {
		nullsOut.Shorten(out.Len())
	}
	return out
}

func (s *sliceExpr) evalStringOrBytesFast(vec vector.Any, from, to int, base1 bool) (vector.Any, bool) {
	switch vec := vec.(type) {
	case *vector.Const:
		slice := vec.Value().Bytes()
		id := vec.Type().ID()
		size := lengthOfBytesOrString(id, slice)
		start, end := 0, size
		if s.fromEval != nil {
			start = sliceIndex(from, size, base1)
		}
		if s.toEval != nil {
			end = sliceIndex(to, size, base1)
		}
		start, end = expr.FixSliceBounds(start, end, size)
		slice = sliceBytesOrString(slice, id, start, end)
		return vector.NewConst(super.NewValue(vec.Type(), slice), vec.Len(), vec.Nulls), true
	case *vector.View:
		out, ok := s.evalStringOrBytesFast(vec.Any, from, to, base1)
		if !ok {
			return nil, false
		}
		return vector.NewView(out, vec.Index), true
	case *vector.Dict:
		out, ok := s.evalStringOrBytesFast(vec.Any, from, to, base1)
		if !ok {
			return nil, false
		}
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls), true
	default:
		offsets, bytes, nullsIn := stringOrBytesContents(vec)
		newOffsets := []uint32{0}
		newBytes := []byte{}
		id := vec.Type().ID()
		for i := range vec.Len() {
			slice := bytes[offsets[i]:offsets[i+1]]
			size := lengthOfBytesOrString(id, slice)
			start, end := 0, size
			if s.fromEval != nil {
				start = sliceIndex(from, size, base1)
			}
			if s.toEval != nil {
				end = sliceIndex(to, size, base1)
			}
			start, end = expr.FixSliceBounds(start, end, size)
			slice = sliceBytesOrString(slice, id, start, end)
			newBytes = append(newBytes, slice...)
			newOffsets = append(newOffsets, newOffsets[len(newOffsets)-1]+uint32(len(slice)))
		}
		return s.bytesOrStringVec(vec.Type(), newOffsets, newBytes, nullsIn), true
	}
}

func (s *sliceExpr) bytesOrStringVec(typ super.Type, offsets []uint32, bytes []byte, nulls bitvec.Bits) vector.Any {
	switch typ.ID() {
	case super.IDBytes:
		return vector.NewBytes(vector.NewBytesTable(offsets, bytes), nulls)
	case super.IDString:
		return vector.NewString(vector.NewBytesTable(offsets, bytes), nulls)
	default:
		panic(typ)
	}
}

func (s *sliceExpr) bytesAt(val vector.Any, slot uint32) ([]byte, bool) {
	switch val := val.(type) {
	case *vector.String:
		if val.Nulls.IsSet(slot) {
			return nil, true
		}
		return val.Table().Bytes(slot), false
	case *vector.Bytes:
		if val.Nulls.IsSet(slot) {
			return nil, true
		}
		return val.Value(slot), false
	case *vector.Const:
		if val.Nulls.IsSet(slot) {
			return nil, true
		}
		s, _ := val.AsBytes()
		return s, false
	case *vector.Dict:
		if val.Nulls.IsSet(slot) {
			return nil, true
		}
		return s.bytesAt(val.Any, uint32(val.Index[slot]))
	case *vector.View:
		return s.bytesAt(val.Any, val.Index[slot])
	}
	panic(val)
}

func lengthOfBytesOrString(id int, slice []byte) int {
	if id == super.IDString {
		return utf8.RuneCount(slice)
	}
	return len(slice)
}

func sliceIsConstIndex(vec vector.Any) (int, bool) {
	if vec == nil {
		return 0, true
	}
	if c, ok := vec.(*vector.Const); ok && c.Nulls.IsZero() {
		return int(c.Value().Int()), true
	}
	return 0, false
}

func sliceIndex(idx, size int, base1 bool) int {
	if base1 && idx > 0 {
		idx--
	}
	if idx < 0 {
		idx += size
	}
	return idx
}

func sliceBytesOrString(slice []byte, id int, start, end int) []byte {
	if id == super.IDString {
		slice = slice[expr.UTF8PrefixLen(slice, start):]
		return slice[:expr.UTF8PrefixLen(slice, end-start)]
	} else {
		return slice[start:end]
	}
}

func stringOrBytesContents(vec vector.Any) ([]uint32, []byte, bitvec.Bits) {
	switch vec := vec.(type) {
	case *vector.String:
		offsets, bytes := vec.Table().Slices()
		return offsets, bytes, vec.Nulls
	case *vector.Bytes:
		offsets, bytes := vec.Table().Slices()
		return offsets, bytes, vec.Nulls
	default:
		panic(vec)
	}
}

func arrayOrSetContents(vec vector.Any) ([]uint32, vector.Any, bitvec.Bits) {
	switch vec := vec.(type) {
	case *vector.Array:
		return vec.Offsets, vec.Values, vec.Nulls
	case *vector.Set:
		return vec.Offsets, vec.Values, vec.Nulls
	default:
		panic(vec)
	}
}

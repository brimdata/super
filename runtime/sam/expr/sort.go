package expr

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/sio"
)

type SortExpr struct {
	Evaluator
	Order order.Which
	Nulls order.Nulls
}

func NewSortExpr(eval Evaluator, o order.Which, n order.Nulls) SortExpr {
	return SortExpr{eval, o, n}
}

// nullsMax reports whether s treats null as the maximum value.
func (s *SortExpr) nullsMax() bool {
	return s.Order == order.Asc && s.Nulls == order.NullsLast ||
		s.Order == order.Desc && s.Nulls == order.NullsFirst
}

func (c *Comparator) sortStableIndices(vals []super.Value) []uint32 {
	if len(c.exprs) == 0 {
		return nil
	}
	n := len(vals)
	if max := math.MaxUint32; n > max {
		panic(fmt.Sprintf("number of values exceeds %d", max))
	}
	indices := make([]uint32, n)
	i64s := make([]int64, n)
	val0s := make([]super.Value, n)
	native := true
	nullsMax0 := c.exprs[0].nullsMax()
	for i := range indices {
		indices[i] = uint32(i)
		val := c.exprs[0].Eval(vals[i])
		val0s[i] = val
		if id := val.Type().ID(); id <= super.IDTime {
			if val.IsNull() {
				if nullsMax0 {
					i64s[i] = math.MaxInt64
				} else {
					i64s[i] = math.MinInt64
				}
			} else if super.IsSigned(id) {
				i64s[i] = val.Int()
			} else {
				v := min(val.Uint(), math.MaxInt64)
				i64s[i] = int64(v)
			}
		} else {
			native = false
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		for k, expr := range c.exprs {
			iidx, jidx := indices[i], indices[j]
			if expr.Order == order.Desc {
				iidx, jidx = jidx, iidx
			}
			var ival, jval super.Value
			if k == 0 {
				if native {
					if i64, j64 := i64s[iidx], i64s[jidx]; i64 != j64 {
						return i64 < j64
					} else if i64 != math.MaxInt64 && i64 != math.MinInt64 {
						continue
					}
				}
				ival, jval = val0s[iidx], val0s[jidx]
			} else {
				ival = expr.Eval(vals[iidx])
				jval = expr.Eval(vals[jidx])
			}
			if v := compareValues(ival, jval, expr.nullsMax()); v != 0 {
				return v < 0
			}
		}
		return false
	})
	return indices
}

type CompareFn func(a, b super.Value) int

func NewValueCompareFn(o order.Which, n order.Nulls) CompareFn {
	return NewComparator(SortExpr{&This{}, o, n}).Compare
}

type Comparator struct {
	exprs []SortExpr
}

// NewComparator returns a super.Value comparator for exprs.  To compare values
// a and b, it iterates over the elements e of exprs, stopping when e(a)!=e(b).
func NewComparator(exprs ...SortExpr) *Comparator {
	return &Comparator{slices.Clone(exprs)}
}

// WithMissingAsNull returns the receiver after modifying it to treat missing
// values as the null value in comparisons.
func (c *Comparator) WithMissingAsNull() *Comparator {
	for i, k := range c.exprs {
		c.exprs[i].Evaluator = &missingAsNull{k}
	}
	return c
}

type missingAsNull struct{ Evaluator }

func (m *missingAsNull) Eval(val super.Value) super.Value {
	val = m.Evaluator.Eval(val)
	if val.IsMissing() {
		return super.Null
	}
	return val
}

// Compare returns an interger comparing two values according to the receiver's
// configuration.  The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
func (c *Comparator) Compare(a, b super.Value) int {
	for _, k := range c.exprs {
		aval := k.Eval(a)
		bval := k.Eval(b)
		if k.Order == order.Desc {
			aval, bval = bval, aval
		}
		if v := compareValues(aval, bval, k.nullsMax()); v != 0 {
			return v
		}
	}
	return 0
}

func compareValues(a, b super.Value, nullsMax bool) int {
	// Handle nulls according to nullsMax
	nullA := a.IsNull()
	nullB := b.IsNull()
	if nullA && nullB {
		return 0
	}
	if nullA {
		if nullsMax {
			return 1
		} else {
			return -1
		}
	}
	if nullB {
		if nullsMax {
			return -1
		} else {
			return 1
		}
	}
	switch aid, bid := a.Type().ID(), b.Type().ID(); {
	case super.IsNumber(aid) && super.IsNumber(bid):
		return compareNumbers(a, b, aid, bid)
	case aid != bid:
		return super.CompareTypes(a.Type(), b.Type())
	case aid == super.IDBool:
		if av, bv := a.Bool(), b.Bool(); av == bv {
			return 0
		} else if av {
			return 1
		}
		return -1
	case aid == super.IDBytes:
		return bytes.Compare(super.DecodeBytes(a.Bytes()), super.DecodeBytes(b.Bytes()))
	case aid == super.IDString:
		return cmp.Compare(super.DecodeString(a.Bytes()), super.DecodeString(b.Bytes()))
	case aid == super.IDIP:
		return super.DecodeIP(a.Bytes()).Compare(super.DecodeIP(b.Bytes()))
	case aid == super.IDType:
		sctx := super.NewContext() // XXX This is expensive.
		// XXX This isn't cheap eventually we should add
		// super.CompareTypeValues(a, b scode.Bytes).
		av, _ := sctx.DecodeTypeValue(a.Bytes())
		bv, _ := sctx.DecodeTypeValue(b.Bytes())
		return super.CompareTypes(av, bv)
	}
	// XXX record support easy to add here if we moved the creation of the
	// field resolvers into this package.
	if innerType := super.InnerType(a.Type()); innerType != nil {
		ait, bit := a.Iter(), b.Iter()
		for {
			if ait.Done() {
				if bit.Done() {
					return 0
				}
				return -1
			}
			if bit.Done() {
				return 1
			}
			aa := super.NewValue(innerType, ait.Next())
			bb := super.NewValue(innerType, bit.Next())
			if v := compareValues(aa, bb, nullsMax); v != 0 {
				return v
			}
		}
	}
	return bytes.Compare(a.Bytes(), b.Bytes())
}

// SortStable sorts vals according to c, with equal values in their original
// order.  SortStable allocates more memory than [SortStableReader].
func (c *Comparator) SortStable(vals []super.Value) {
	tmp := make([]super.Value, len(vals))
	for i, index := range c.sortStableIndices(vals) {
		tmp[i] = vals[i]
		if j := int(index); i < j {
			vals[i] = vals[j]
		} else if i > j {
			vals[i] = tmp[j]
		}
	}
}

// SortStableReader returns a reader for vals sorted according to c, with equal
// values in their original order.
func (c *Comparator) SortStableReader(vals []super.Value) sio.Reader {
	return &sortStableReader{
		indices: c.sortStableIndices(vals),
		vals:    vals,
	}
}

type sortStableReader struct {
	indices []uint32
	vals    []super.Value
}

func (s *sortStableReader) Read() (*super.Value, error) {
	if len(s.indices) == 0 {
		return nil, nil
	}
	val := &s.vals[s.indices[0]]
	s.indices = s.indices[1:]
	return val, nil
}

type RecordSlice struct {
	vals    []super.Value
	compare CompareFn
}

func NewRecordSlice(compare CompareFn) *RecordSlice {
	return &RecordSlice{compare: compare}
}

// Swap implements sort.Interface for *Record slices.
func (r *RecordSlice) Len() int { return len(r.vals) }

// Swap implements sort.Interface for *Record slices.
func (r *RecordSlice) Swap(i, j int) { r.vals[i], r.vals[j] = r.vals[j], r.vals[i] }

// Less implements sort.Interface for *Record slices.
func (r *RecordSlice) Less(i, j int) bool {
	return r.compare(r.vals[i], r.vals[j]) < 0
}

// Push adds x as element Len(). Implements heap.Interface.
func (r *RecordSlice) Push(rec any) {
	r.vals = append(r.vals, rec.(super.Value))
}

// Pop removes the first element in the array. Implements heap.Interface.
func (r *RecordSlice) Pop() any {
	rec := r.vals[len(r.vals)-1]
	r.vals = r.vals[:len(r.vals)-1]
	return rec
}

// Index returns the ith record.
func (r *RecordSlice) Index(i int) super.Value {
	return r.vals[i]
}

package expr

import (
	"bytes"
	"sort"

	"github.com/mccanne/zq/zbuf"
	"github.com/mccanne/zq/zcode"
	"github.com/mccanne/zq/zng"
)

type SortFn func(a *zbuf.Record, b *zbuf.Record) int

// Internal function that compares two values of compatible types.
type comparefn func(a, b zcode.Bytes) int

func isNull(val zng.Value) bool {
	return val.Type == nil || val.Bytes == nil
}

// NewSortFn creates a function that compares a pair of Records
// based on the provided ordered list of fields.
// The returned function uses the same return conventions as standard
// routines such as bytes.Compare() and strings.Compare(), so it may
// be used with packages such as heap and sort.
// The handling of records in which a comparison field is unset or not
// present (collectively refered to as fields in which the value is "null")
// is governed by the nullsMax parameter.  If this parameter is true,
// a record with a null value is considered larger than a record with any
// other value, and vice versa.
func NewSortFn(nullsMax bool, fields ...FieldExprResolver) SortFn {
	sorters := make(map[zng.Type]comparefn)
	return func(ra *zbuf.Record, rb *zbuf.Record) int {
		for _, resolver := range fields {
			a := resolver(ra)
			b := resolver(rb)

			// Handle nulls according to nullsMax
			nullA := isNull(a)
			nullB := isNull(b)
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

			// If values are of different types, just compare
			// the native representation of the type
			if !zng.SameType(a.Type, b.Type) {
				return bytes.Compare([]byte(a.Type.String()), []byte(b.Type.String()))
			}

			sf, ok := sorters[a.Type]
			if !ok {
				sf = lookupSorter(a.Type)
				sorters[a.Type] = sf
			}

			v := sf(a.Bytes, b.Bytes)
			// If the events don't match, then return the sort
			// info.  Otherwise, they match and we continue on
			// on in the loop to the secondary key, etc.
			if v != 0 {
				return v
			}
		}
		// All the keys matched with equality.
		return 0
	}
}

// SortStable performs a stable sort on the provided records.
func SortStable(records []*zbuf.Record, sorter SortFn) {
	slice := &RecordSlice{records, sorter}
	sort.Stable(slice)
}

type RecordSlice struct {
	records []*zbuf.Record
	sorter  SortFn
}

func NewRecordSlice(sorter SortFn) *RecordSlice {
	return &RecordSlice{sorter: sorter}
}

// Swap implements sort.Interface for *Record slices.
func (r *RecordSlice) Len() int { return len(r.records) }

// Swap implements sort.Interface for *Record slices.
func (r *RecordSlice) Swap(i, j int) { r.records[i], r.records[j] = r.records[j], r.records[i] }

// Less implements sort.Interface for *Record slices.
func (r *RecordSlice) Less(i, j int) bool {
	return r.sorter(r.records[i], r.records[j]) <= 0
}

// Push adds x as element Len(). Implements heap.Interface.
func (r *RecordSlice) Push(rec interface{}) {
	r.records = append(r.records, rec.(*zbuf.Record))
}

// Pop removes the first element in the array. Implements heap.Interface.
func (r *RecordSlice) Pop() interface{} {
	rec := r.records[len(r.records)-1]
	r.records = r.records[:len(r.records)-1]
	return rec
}

// Index returns the ith record.
func (r *RecordSlice) Index(i int) *zbuf.Record {
	return r.records[i]
}

func lookupSorter(typ zng.Type) comparefn {
	// XXX record support easy to add here if we moved the creation of the
	// field resolvers into this package.
	if innerType := zng.InnerType(typ); innerType != nil {
		return func(a, b zcode.Bytes) int {
			compare := lookupSorter(innerType)
			ia := a.Iter()
			ib := b.Iter()
			for {
				if ia.Done() {
					if ib.Done() {
						return 0
					}
					return -1
				}
				if ib.Done() {
					return 1
				}
				va, container, err := ia.Next()
				if container || err != nil {
					return -1
				}

				vb, container, err := ib.Next()
				if container || err != nil {
					return 1
				}
				if v := compare(va, vb); v != 0 {
					return v
				}
			}
		}
	}
	switch typ {
	default:
		return func(a, b zcode.Bytes) int {
			return bytes.Compare(a, b)
		}
	case zng.TypeBool:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeBool(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeBool(b)
			if err != nil {
				return 1
			}
			if va == vb {
				return 0
			}
			if va {
				return 1
			}
			return -1
		}

	case zng.TypeString, zng.TypeEnum:
		return func(a, b zcode.Bytes) int {
			return bytes.Compare(a, b)
		}

	//XXX note zeek port type can have "/tcp" etc suffix according
	// to docs but we've only encountered ints in data files.
	// need to fix this.  XXX also we should break this sorts
	// into the different types.
	case zng.TypeInt:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeInt(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeInt(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}

	case zng.TypeCount:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeCount(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeCount(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case zng.TypePort:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodePort(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodePort(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}

	case zng.TypeDouble:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeDouble(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeDouble(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}

	case zng.TypeTime:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeTime(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeTime(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}

	case zng.TypeInterval:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeInterval(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeInterval(b)
			if err != nil {
				return 1
			}
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}

	case zng.TypeAddr:
		return func(a, b zcode.Bytes) int {
			va, err := zng.DecodeAddr(a)
			if err != nil {
				return -1
			}
			vb, err := zng.DecodeAddr(b)
			if err != nil {
				return 1
			}
			return bytes.Compare(va.To16(), vb.To16())
		}
	}
}

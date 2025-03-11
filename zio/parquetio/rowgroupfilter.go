package parquetio

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"strings"

	"github.com/apache/arrow-go/v18/parquet/metadata"
	"github.com/apache/arrow-go/v18/parquet/schema"
	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/zson"
	"github.com/x448/float16"
)

type rowGroupFilter struct {
	rgMetadata *metadata.RowGroupMetaData
	schema     *schema.Schema
	zctx       *super.Context
}

func (r *rowGroupFilter) evalFilter(e dag.Expr) (match, ok bool) {
	switch e := e.(type) {
	case *dag.BinaryExpr:
		switch e.Op {
		case "and":
			if match, ok := r.evalFilter(e.LHS); !match && ok {
				return false, true
			}
			r.evalFilter(e.RHS)
		case "or":
			if match, ok := r.evalFilter(e.LHS); match || !ok {
				return match, ok
			}
			return r.evalFilter(e.RHS)
		case "in":
			var elems []dag.VectorElem
			switch e := e.RHS.(type) {
			case *dag.ArrayExpr:
				elems = e.Elems
			case *dag.SetExpr:
				elems = e.Elems
			default:
				return false, false
			}
			for _, elem := range elems {
				vv, ok := elem.(*dag.VectorValue)
				if !ok {
					return false, false
				}
				if match, ok := r.evalComparison("==", e.LHS, vv.Expr); match || !ok {
					return match, ok
				}
			}
			return false, true
		case "==", "!=", "<", "<=", ">", ">=":
			return r.evalComparison(e.Op, e.LHS, e.RHS)
		}
	case *dag.UnaryExpr:
		if e.Op == "!" {
			match, ok := r.evalFilter(e.Operand)
			return !match, ok
		}
	}
	return false, false
}

func (r *rowGroupFilter) evalComparison(op string, lhs, rhs dag.Expr) (match, ok bool) {
	if _, ok := lhs.(*dag.This); !ok {
		if op[0] == '<' {
			op = strings.ReplaceAll(op, "<", ">")
		} else {
			op = strings.ReplaceAll(op, ">", "<")
		}
		lhs, rhs = rhs, lhs
	}
	lhsThis, ok1 := lhs.(*dag.This)
	rhsLiteral, ok2 := rhs.(*dag.Literal)
	if !ok1 || !ok2 {
		return false, false
	}
	col := r.schema.ColumnIndexByName(strings.Join(lhsThis.Path, "."))
	if col < 0 {
		return false, false
	}
	min, max, ok := columnChunkStats(r.rgMetadata, col)
	if !ok {
		return false, false
	}
	val, err := zson.ParseValue(r.zctx, rhsLiteral.Value)
	if err != nil {
		return false, false
	}
	minResult, ok1 := compare(val, min)
	maxResult, ok2 := compare(val, max)
	if !ok1 || !ok2 {
		return false, false
	}
	switch op {
	case "==":
		// val >= min && val <= max
		return minResult >= 0 && maxResult <= 0, true
	case "!=":
		// val < min || val > max
		return minResult < 0 || maxResult > 0, true
	case "<":
		// val < max
		return maxResult < 0, true
	case "<=":
		// val <= max
		return maxResult <= 0, true
	case ">":
		// val > min
		return minResult > 0, true
	case ">=":
		// val >= min
		return minResult >= 0, true
	}
	panic(op)
}

func compare(aVal super.Value, b any) (int, bool) {
	if aVal.IsNull() {
		return 0, false
	}
	switch b := b.(type) {
	case bool:
		if aVal.Type().ID() != super.IDBool {
			return 0, false
		}
		if a := aVal.Bool(); a == b {
			return 0, true
		} else if a {
			return -1, true
		}
		return 1, true
	case []byte:
		if id := aVal.Type().ID(); id != super.IDBytes && id != super.IDString {
			return 0, false
		}
		a := aVal.Bytes()
		n := min(len(a), len(b))
		return bytes.Compare(a[:n], b[:n]), true
	case float64:
		a, ok := coerce.ToFloat(aVal, super.TypeFloat64)
		return cmp.Compare(a, b), ok
	case int64:
		a, ok := coerce.ToInt(aVal, super.TypeInt64)
		return cmp.Compare(a, b), ok
	case uint64:
		a, ok := coerce.ToUint(aVal, super.TypeUint64)
		return cmp.Compare(a, b), ok
	default:
		panic(b)
	}
}

func columnChunkStats(rgmd *metadata.RowGroupMetaData, col int) (min, max any, ok bool) {
	ccmd, err := rgmd.ColumnChunk(col)
	if err != nil {
		return nil, nil, false
	}
	stats, err := ccmd.Statistics()
	if stats == nil || !stats.HasMinMax() || err != nil {
		return nil, nil, false
	}
	switch stats := stats.(type) {
	case *metadata.BooleanStatistics:
		return stats.Min(), stats.Max(), true
	case *metadata.ByteArrayStatistics:
		return stats.Min().Bytes(), stats.Max().Bytes(), true
	case *metadata.Float16Statistics:
		min := float16.Frombits(binary.LittleEndian.Uint16(stats.Min())).Float32()
		max := float16.Frombits(binary.LittleEndian.Uint16(stats.Max())).Float32()
		return float64(min), float64(max), true
	case *metadata.Float32Statistics:
		return float64(stats.Min()), float64(stats.Max()), true
	case *metadata.Float64Statistics:
		return stats.Min(), stats.Max(), true
	case *metadata.Int32Statistics:
		if stats.Descr().SortOrder() == schema.SortUNSIGNED {
			return uint64(stats.Min()), uint64(stats.Max()), true
		}
		return uint64(stats.Min()), uint64(stats.Max()), true
	case *metadata.Int64Statistics:
		if stats.Descr().SortOrder() == schema.SortUNSIGNED {
			return uint64(stats.Min()), uint64(stats.Max()), true
		}
		return stats.Min(), stats.Max(), true
	case *metadata.Int96Statistics:
		return nil, nil, false
	}
	panic(stats)
}

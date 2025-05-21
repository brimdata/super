package parquetio

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet/metadata"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/x448/float16"
)

func buildPrunerValue(sctx *super.Context, rgmd *metadata.RowGroupMetaData, colIndexes []int, colIndexToField map[int]*pqarrow.SchemaField) super.Value {
	var paths field.List
	var vals []super.Value
	m := sup.NewBSUPMarshaler()
	for _, i := range colIndexes {
		min, max, path, ok := columnChunkStats(rgmd, i, colIndexToField[i].Field.Type)
		if !ok {
			continue
		}
		minPath := append(slices.Clone(path), "min")
		maxPath := append(slices.Clone(path), "max")
		minVal, err := m.Marshal(min)
		if err != nil {
			panic(err)
		}
		maxVal, err := m.Marshal(max)
		if err != nil {
			panic(err)
		}
		paths = append(paths, minPath, maxPath)
		vals = append(vals, minVal, maxVal)
	}
	b, err := super.NewRecordBuilder(sctx, paths)
	if err != nil {
		panic(err)
	}
	var types []super.Type
	for _, val := range vals {
		types = append(types, val.Type())
		b.Append(val.Bytes())
	}
	bytes, err := b.Encode()
	if err != nil {
		panic(err)
	}
	return super.NewValue(b.Type(types), bytes)
}

func columnChunkStats(rgmd *metadata.RowGroupMetaData, col int, dt arrow.DataType) (min, max any, path []string, ok bool) {
	ccmd, err := rgmd.ColumnChunk(col)
	if err != nil {
		return nil, nil, nil, false
	}
	stats, err := ccmd.Statistics()
	if stats == nil || !stats.HasMinMax() || err != nil {
		return nil, nil, nil, false
	}
	path = stats.Descr().ColumnPath()
	switch stats := stats.(type) {
	case *metadata.BooleanStatistics:
		return stats.Min(), stats.Max(), path, true
	case *metadata.ByteArrayStatistics:
		if id := dt.ID(); id == arrow.STRING || id == arrow.LARGE_STRING {
			return string(stats.Min()), string(stats.Max()), path, true
		}
		return stats.Min(), stats.Max(), path, true
	case *metadata.Float16Statistics:
		min := float16.Frombits(binary.LittleEndian.Uint16(stats.Min()))
		max := float16.Frombits(binary.LittleEndian.Uint16(stats.Max()))
		return min.Float32(), max.Float32(), path, true
	case *metadata.Float32Statistics:
		return stats.Min(), stats.Max(), path, true
	case *metadata.Float64Statistics:
		return stats.Min(), stats.Max(), path, true
	case *metadata.Int32Statistics:
		if arrow.IsUnsignedInteger(dt.ID()) {
			return uint32(stats.Min()), uint32(stats.Max()), path, true
		}
		multiplier := multiplier(dt)
		return int64(stats.Min()) * multiplier, int64(stats.Max()) * multiplier, path, true
	case *metadata.Int64Statistics:
		if arrow.IsUnsignedInteger(dt.ID()) {
			return uint64(stats.Min()), uint64(stats.Max()), path, true
		}
		multiplier := multiplier(dt)
		return stats.Min() * multiplier, stats.Max() * multiplier, path, true
	case *metadata.Int96Statistics:
		return nil, nil, nil, false
	}
	panic(stats)
}

func multiplier(dt arrow.DataType) int64 {
	if dt.ID() == arrow.DATE32 {
		return int64(24 * time.Hour)
	}
	if twu, ok := dt.(arrow.TemporalWithUnit); ok {
		return int64(twu.TimeUnit().Multiplier())
	}
	return 1
}

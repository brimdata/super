package parquetio

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

type builder struct {
	zcode.Builder
	buf []byte
}

func (b *builder) appendValue(typ zed.Type, v interface{}) {
	switch v := v.(type) {
	case nil:
		b.AppendNull()
	case []byte:
		b.AppendPrimitive(v)
	case bool:
		b.buf = zed.AppendBool(b.buf[:0], v)
		b.AppendPrimitive(b.buf)
	case float32:
		b.buf = zed.AppendFloat32(b.buf[:0], v)
		b.AppendPrimitive(b.buf)
	case float64:
		b.buf = zed.AppendFloat64(b.buf[:0], v)
		b.AppendPrimitive(b.buf)
	case int32:
		b.buf = zed.AppendInt(b.buf[:0], int64(v))
		b.AppendPrimitive(b.buf)
	case int64:
		b.buf = zed.AppendInt(b.buf[:0], v)
		b.AppendPrimitive(b.buf)
	case uint32:
		b.buf = zed.AppendUint(b.buf[:0], uint64(v))
		b.AppendPrimitive(b.buf)
	case uint64:
		b.buf = zed.AppendUint(b.buf[:0], v)
		b.AppendPrimitive(b.buf)
	case map[string]interface{}:
		switch typ := zed.AliasOf(typ).(type) {
		case *zed.TypeArray:
			switch v := v["list"].(type) {
			case nil:
				b.AppendNull()
			case []map[string]interface{}:
				b.BeginContainer()
				for _, m := range v {
					b.appendValue(typ.Type, m["element"])
				}
				b.EndContainer()
			default:
				panic(fmt.Sprintf("unknown type %T", v))
			}
		case *zed.TypeMap:
			switch v := v["key_value"].(type) {
			case nil:
				b.AppendNull()
			case []map[string]interface{}:
				b.BeginContainer()
				for _, m := range v {
					b.appendValue(typ.KeyType, m["key"])
					b.appendValue(typ.ValType, m["value"])
				}
				b.EndContainer()
			default:
				panic(fmt.Sprintf("unknown type %T", v))
			}
		case *zed.TypeRecord:
			b.BeginContainer()
			for _, c := range typ.Columns {
				b.appendValue(c.Type, v[c.Name])
			}
			b.EndContainer()
		default:
			panic(fmt.Sprintf("unknown type %T", typ))
		}
	default:
		panic(fmt.Sprintf("unknown type %T", v))
	}
}

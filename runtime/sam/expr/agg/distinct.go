package agg

import (
	"encoding/binary"
	"fmt"

	"github.com/superdb/super"
	"github.com/superdb/super/scode"
)

func DistinctResultAsPartial(sctx *super.Context, seen map[string]struct{}) super.Value {
	vals := make([]super.Value, 0, len(seen))
	for key := range seen {
		vals = append(vals, NewValueFromDistinctKey(sctx, key))
		delete(seen, key)
	}
	return newArray(sctx, vals)
}

func NewValueFromDistinctKey(sctx *super.Context, key string) super.Value {
	bytes := []byte(key)
	id, n := binary.Varint(bytes)
	if n <= 0 {
		panic(fmt.Sprintf("bad varint: %d", n))
	}
	bytes = bytes[n:]
	typ, err := sctx.LookupType(int(id))
	if err != nil {
		panic(err)
	}
	return super.NewValue(typ, scode.Bytes(bytes).Body())
}

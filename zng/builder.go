package zng

import (
	"github.com/brimsec/zq/zcode"
)

// Builder provides a way of easily and efficiently building records
// of the same type.
type Builder struct {
	zcode.Builder
	Type *TypeRecord
	rec  Record
}

func NewBuilder(typ *TypeRecord) *Builder {
	return &Builder{Type: typ}
}

// Build encodes the top-level zcode.Bytes values as the Raw field
// of a record and sets that field and the Type field of the passed-in record.
// XXX This currently only works for zvals that are properly formatted for
// the top-level scan of the record, e.g., if a field is record[id:[record:[orig_h:ip]]
// then the zval passed in here for that field must have the proper encoding...
// this works fine when values are extracted and inserted from the proper level
// but when leaf values are inserted we should have another method to handle this,
// e.g., by encoding the dfs traversal of the record type with info about
// primitive vs container insertions.  This could be the start of a whole package
// that provides different ways to build zng.Records via, e.g., a marshal API,
// auto-generated stubs, etc.
func (b *Builder) Build(zvs ...zcode.Bytes) *Record {
	b.Reset()
	cols := b.Type.Columns
	for k, zv := range zvs {
		if IsContainerType(cols[k].Type) {
			b.AppendContainer(zv)
		} else {
			b.AppendPrimitive(zv)
		}
	}
	// Note that t.rec.nonvolatile is false so anything downstream
	// will have to copy the record and we can re-use the record value
	// between subsequent calls.
	b.rec.Type = b.Type
	b.rec.Raw = b.Bytes()
	// fill in the ts if there's a ts field
	if b.Type.TsCol >= 0 {
		b.rec.Ts, _ = b.rec.AccessTime("ts")
	}
	return &b.rec
}

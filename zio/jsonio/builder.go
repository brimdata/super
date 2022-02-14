package jsonio

import (
	"errors"
	"sort"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

type builder struct {
	zctx *zed.Context

	containers []int  // Stack of open containers (as indexes into items).
	items      []item // Stack of items.

	// These exist only to reduce memory allocations.
	bytes    []byte
	columns  []zed.Column
	itemptrs []*item
	types    []zed.Type
}

type item struct {
	fieldName string
	typ       zed.Type
	zb        zcode.Builder
}

func (b *builder) reset() {
	b.containers = b.containers[:0]
	b.items = b.items[:0]
}

func (b *builder) pushPrimitiveItem(fieldName string, typ zed.Type, bytes zcode.Bytes) {
	i := b.pushItem(fieldName)
	i.typ = typ
	i.zb.Append(bytes)
}

func (b *builder) pushItem(fieldName string) *item {
	n := len(b.items)
	if n == cap(b.items) {
		b.items = append(b.items, item{})
	}
	b.items = b.items[:n+1]
	i := &b.items[n]
	i.fieldName = fieldName
	i.zb.Truncate()
	return i
}

func (b *builder) beginContainer(fieldName string) {
	// This item represents the container.  endArray or endRecord will set
	// its type and bytes.
	b.pushItem(fieldName)
	b.containers = append(b.containers, len(b.items))
}

func (b *builder) endContainer() (container *item, items []item) {
	start := b.containers[len(b.containers)-1]
	b.containers = b.containers[:len(b.containers)-1]
	items = b.items[start:]
	b.items = b.items[:start]
	return &b.items[start-1], items
}

func (b *builder) endArray() {
	container, items := b.endContainer()

	b.types = b.types[:0]
	for i := range items {
		if items[i].typ != zed.TypeNull {
			b.types = append(b.types, items[i].typ)
		}
	}
	sort.Slice(b.types, func(i, j int) bool { return b.types[i].ID() < b.types[j].ID() })
	dedupedTypes := b.types[:0]
	var prev zed.Type
	for _, t := range b.types {
		// JSON doesn't use named types, so even though b.types was
		// sorted by zed.Type.ID above, we can compare elements directly
		// without calling zed.TypeUnder.
		if t != prev {
			dedupedTypes = append(dedupedTypes, t)
			prev = t
		}
	}
	b.types = dedupedTypes

	container.zb.BeginContainer()
	switch len(b.types) {
	case 0:
		container.typ = b.zctx.LookupTypeArray(zed.TypeNull)
		for range items {
			container.zb.Append(nil)
		}
	case 1:
		container.typ = b.zctx.LookupTypeArray(b.types[0])
		for i := range items {
			container.zb.Append(items[i].zb.Bytes().Body())
		}
	default:
		union := b.zctx.LookupTypeUnion(b.types)
		if &union.Types[0] == &b.types[0] {
			// union now owns b.types, so don't reuse it.
			b.types = nil
		}
		container.typ = b.zctx.LookupTypeArray(union)
		for i := range items {
			if bytes := items[i].zb.Bytes().Body(); bytes == nil {
				container.zb.Append(nil)
			} else {
				selector := union.Selector(items[i].typ)
				zed.BuildUnion(&container.zb, selector, bytes)
			}
		}
	}
	container.zb.EndContainer()
}

func (b *builder) endRecord() {
	container, items := b.endContainer()
	b.itemptrs = b.itemptrs[:0]
	for i := range items {
		b.itemptrs = append(b.itemptrs, &items[i])
	}
	itemptrs := b.itemptrs
	for {
		b.columns = b.columns[:0]
		for _, item := range itemptrs {
			b.columns = append(b.columns, zed.NewColumn(item.fieldName, item.typ))
		}
		var err error
		container.typ, err = b.zctx.LookupTypeRecord(b.columns)
		if err == nil {
			break
		}
		var dferr *zed.DuplicateFieldError
		if !errors.As(err, &dferr) {
			panic(err)
		}
		// removeDuplicateItems operates on itemptrs rather than items
		// to avoid copying zcode.Builders.
		itemptrs = removeDuplicateItems(itemptrs, dferr.Name)
	}
	container.zb.BeginContainer()
	for _, item := range itemptrs {
		container.zb.Append(item.zb.Bytes().Body())
	}
	container.zb.EndContainer()
}

// removeDuplicateItems removes from itemptrs any item whose fieldName field
// equals name except for the last such item, which it moves to the position at
// which it found the first such item.  (This behavior mimics jq's handling of
// duplicate JSON object fields.)
func removeDuplicateItems(itemptrs []*item, name string) []*item {
	out := itemptrs[:0]
	var first = -1
	for i, item := range itemptrs {
		if item.fieldName == name {
			if first >= 0 {
				out[first] = item
				continue
			}
			first = i
		}
		out = append(out, item)
	}
	return out
}

func (b *builder) value() *zed.Value {
	if len(b.containers) > 0 {
		panic("open container")
	}
	if len(b.items) > 1 {
		panic("multiple items")
	}
	bytes := b.items[0].zb.Bytes().Body()
	// Reset gives us ownership of bytes.
	b.items[0].zb.Reset()
	return zed.NewValue(b.items[0].typ, bytes)
}

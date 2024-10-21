package jsonio

import (
	"errors"
	"slices"
	"sort"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type builder struct {
	zctx *super.Context

	containers []int  // Stack of open containers (as indexes into items).
	items      []item // Stack of items.

	// These exist only to reduce memory allocations.
	fields   []super.Field
	itemptrs []*item
	types    []super.Type
	val      super.Value
}

type item struct {
	fieldName string
	typ       super.Type
	zb        zcode.Builder
}

func (b *builder) reset() {
	b.containers = b.containers[:0]
	b.items = b.items[:0]
}

func (b *builder) pushPrimitiveItem(fieldName string, typ super.Type, bytes zcode.Bytes) {
	i := b.pushItem(fieldName)
	i.typ = typ
	i.zb.Append(bytes)
}

func (b *builder) pushItem(fieldName string) *item {
	n := len(b.items)
	b.items = slices.Grow(b.items, 1)[:n+1]
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
		if items[i].typ != super.TypeNull {
			b.types = append(b.types, items[i].typ)
		}
	}
	sort.Slice(b.types, func(i, j int) bool { return b.types[i].ID() < b.types[j].ID() })
	dedupedTypes := b.types[:0]
	var prev super.Type
	for _, t := range b.types {
		// JSON doesn't use named types, so even though b.types was
		// sorted by super.Type.ID above, we can compare elements directly
		// without calling super.TypeUnder.
		if t != prev {
			dedupedTypes = append(dedupedTypes, t)
			prev = t
		}
	}
	b.types = dedupedTypes

	container.zb.BeginContainer()
	switch len(b.types) {
	case 0:
		container.typ = b.zctx.LookupTypeArray(super.TypeNull)
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
		container.typ = b.zctx.LookupTypeArray(union)
		for i := range items {
			if bytes := items[i].zb.Bytes().Body(); bytes == nil {
				container.zb.Append(nil)
			} else {
				tag := union.TagOf(items[i].typ)
				super.BuildUnion(&container.zb, tag, bytes)
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
		b.fields = b.fields[:0]
		for _, item := range itemptrs {
			b.fields = append(b.fields, super.NewField(item.fieldName, item.typ))
		}
		var err error
		container.typ, err = b.zctx.LookupTypeRecord(b.fields)
		if err == nil {
			break
		}
		var dferr *super.DuplicateFieldError
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
// which it found the first such item.  (This is how both ECMAScript 2015 and jq
// handle duplicate object keys.)
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

func (b *builder) value() *super.Value {
	if len(b.containers) > 0 {
		panic("open container")
	}
	if len(b.items) > 1 {
		panic("multiple items")
	}
	item := &b.items[0]
	b.val = super.NewValue(item.typ, item.zb.Bytes().Body())
	return &b.val
}

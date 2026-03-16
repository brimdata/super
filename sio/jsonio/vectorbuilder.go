package jsonio

import (
	"encoding/json"
)

// Implements the ast.Visitor
type Builder struct {
	root *Value
	// firstRecordCol only exists for the first key in a record to make sure
	// it returns the current root. It's not great, is there a better way?!
	firstRecordCol *Value
}

func NewBuilder() *Builder {
	b := &Builder{root: NewColumn(nil), firstRecordCol: NewColumn(nil)}
	return b
}

func (b *Builder) OnNull() error {
	b.root.Nulls++
	b.root.tags = append(b.root.tags, Null)
	return nil
}

func (b *Builder) OnBool(v bool) error {
	panic("TBD") // XXX need functionality to grow BitVec.
}

func (b *Builder) OnString(v string) error {
	b.root.tags = append(b.root.tags, String)
	b.root.Strings.Append(v)
	return nil
}

func (b *Builder) OnInt64(v int64, n json.Number) error {
	b.root.tags = append(b.root.tags, Int)
	b.root.Ints.Append(v)
	return nil
}

// // OnFloat64 handles a JSON number value with float64 type.
func (b *Builder) OnFloat64(v float64, n json.Number) error {
	b.root.tags = append(b.root.tags, Float)
	b.root.Floats.Append(v)
	return nil
}

// OnObjectBegin handles the beginning of a JSON object value with a
// suggested capacity that can be used to make your custom object container.
//
// After this point the visitor will receive a sequence of callbacks like
// [string, value, string, value, ......, ObjectEnd].
//
// Note:
// 1. This is a recursive definition which means the value can
// also be a JSON object / array described by a sequence of callbacks.
// 2. The suggested capacity will be 0 if current object is empty.
// 3. Currently sonic use a fixed capacity for non-empty object (keep in
// sync with ast.Node) which might not be very suitable. This may be
// improved in future version.
func (b *Builder) OnObjectBegin(capacity int) error {
	if b.root.Object == nil {
		b.root.Object = NewObjectColumn()
	}
	b.root.tags = append(b.root.tags, Object)
	b.firstRecordCol.Parent = b.root
	b.root = b.firstRecordCol
	return nil
}

// // OnObjectKey handles a JSON object key string in member.
func (b *Builder) OnObjectKey(key string) error {
	b.root = b.root.Parent
	// XXX We need to update presence for this particular field.
	b.root = b.root.Object.Lookup(b.root, key)
	return nil
}

// // OnObjectEnd handles the ending of a JSON object value.
func (b *Builder) OnObjectEnd() error {
	b.root = b.root.Parent
	return nil
}

// OnArrayBegin handles the beginning of a JSON array value with a
// suggested capacity that can be used to make your custom array container.
//
// After this point the visitor will receive a sequence of callbacks like
// [value, value, value, ......, ArrayEnd].
//
// Note:
// 1. This is a recursive definition which means the value can
// also be a JSON object / array described by a sequence of callbacks.
// 2. The suggested capacity will be 0 if current array is empty.
// 3. Currently sonic use a fixed capacity for non-empty array (keep in
// sync with ast.Node) which might not be very suitable. This may be
// improved in future version.
func (b *Builder) OnArrayBegin(capacity int) error {
	if b.root.Array == nil {
		b.root.Array = NewArrayColumn(b.root)
	}
	b.root = b.root.Array.element
	b.root.tags = append(b.root.tags, Array)
	return nil
}

func (b *Builder) OnArrayEnd() error {
	b.root = b.root.Parent
	// append offsets
	b.root.Array.offsets = append(b.root.Array.offsets, uint32(len(b.root.Array.element.tags)))
	return nil
}

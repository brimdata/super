package zng

import (
	"fmt"
	"strings"

	"github.com/brimdata/zed/zcode"
)

type TypeUnion struct {
	id    int
	Types []Type
}

func NewTypeUnion(id int, types []Type) *TypeUnion {
	return &TypeUnion{id, types}
}

func (t *TypeUnion) ID() int {
	return t.id
}

// Type returns the type corresponding to selector.
func (t *TypeUnion) Type(selector int) (Type, error) {
	if selector < 0 || selector >= len(t.Types) {
		return nil, ErrUnionSelector
	}
	return t.Types[selector], nil
}

// SplitZng takes a zng encoding of a value of the receiver's union type and
// returns the concrete type of the value, its selector, and the value encoding.
func (t *TypeUnion) SplitZng(zv zcode.Bytes) (Type, int64, zcode.Bytes, error) {
	it := zv.Iter()
	v, container, err := it.Next()
	if err != nil {
		return nil, -1, nil, err
	}
	if container {
		return nil, -1, nil, ErrBadValue
	}
	selector, err := DecodeInt(v)
	if err != nil {
		return nil, -1, nil, err
	}
	inner, err := t.Type(int(selector))
	if err != nil {
		return nil, -1, nil, err
	}
	v, _, err = it.Next()
	if err != nil {
		return nil, -1, nil, err
	}
	if !it.Done() {
		return nil, -1, nil, ErrBadValue
	}
	return inner, int64(selector), v, nil
}

func (t *TypeUnion) Marshal(zv zcode.Bytes) (interface{}, error) {
	inner, _, zv, err := t.SplitZng(zv)
	if err != nil {
		return nil, err
	}
	return Value{inner, zv}, nil
}

func (t *TypeUnion) String() string {
	var ss []string
	for _, typ := range t.Types {
		ss = append(ss, typ.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(ss, ","))
}

func (t *TypeUnion) Format(zv zcode.Bytes) string {
	typ, _, iv, err := t.SplitZng(zv)
	if err != nil {
		return badZng(err, t, zv)
	}
	return fmt.Sprintf("%s (%s) %s", typ.Format(iv), typ, t)
}

// BuildUnion appends to b a union described by selector, val, and container.
func BuildUnion(b *zcode.Builder, selector int, val zcode.Bytes, container bool) {
	if val == nil {
		b.AppendNull()
		return
	}
	b.BeginContainer()
	b.AppendPrimitive(EncodeInt(int64(selector)))
	if container {
		b.AppendContainer(val)
	} else {
		b.AppendPrimitive(val)
	}
	b.EndContainer()
}

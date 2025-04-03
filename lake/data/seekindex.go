package data

import (
	"context"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake/seekindex"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zio/zngio"
)

func LookupSeekRange(ctx context.Context, engine storage.Engine, path *storage.URI,
	obj *Object, pruner expr.Evaluator) ([]seekindex.Range, error) {
	if pruner == nil {
		// scan whole object
		return nil, nil
	}
	r, err := engine.Get(ctx, obj.SeekIndexURI(path))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var ranges seekindex.Ranges
	unmarshaler := sup.NewZNGUnmarshaler()
	reader := zngio.NewReader(super.NewContext(), r)
	defer reader.Close()
	ectx := expr.NewContext()
	for {
		val, err := reader.Read()
		if val == nil || err != nil {
			return ranges, err
		}
		result := pruner.Eval(ectx, *val)
		if result.Type() == super.TypeBool && result.Bool() {
			continue
		}
		var entry seekindex.Entry
		if err := unmarshaler.Unmarshal(*val, &entry); err != nil {
			return nil, fmt.Errorf("corrupt seek index entry for %q at value: %q (%w)", obj.ID.String(), sup.String(val), err)
		}
		ranges.Append(entry)
	}
}

func RangeFromBitVector(ctx context.Context, engine storage.Engine, path *storage.URI,
	o *Object, b *vector.Bool) ([]seekindex.Range, error) {
	index, err := readSeekIndex(ctx, engine, path, o)
	if err != nil {
		return nil, err
	}
	return index.Filter(b), nil
}

func readSeekIndex(ctx context.Context, engine storage.Engine, path *storage.URI, o *Object) (seekindex.Index, error) {
	r, err := engine.Get(ctx, o.SeekIndexURI(path))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	zr := zngio.NewReader(super.NewContext(), r)
	u := sup.NewZNGUnmarshaler()
	var index seekindex.Index
	for {
		val, err := zr.Read()
		if val == nil {
			return index, err
		}
		var entry seekindex.Entry
		if err := u.Unmarshal(*val, &entry); err != nil {
			return nil, err
		}
		index = append(index, entry)
	}
}

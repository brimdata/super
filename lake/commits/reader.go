package commits

import (
	"context"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

type LogReader struct {
	ctx       context.Context
	marshaler *sup.MarshalBSUPContext
	store     *Store
	cursor    ksuid.KSUID
	stop      ksuid.KSUID
}

var _ zio.Reader = (*LogReader)(nil)

func newLogReader(ctx context.Context, zctx *super.Context, store *Store, leaf, stop ksuid.KSUID) *LogReader {
	m := sup.NewBSUPMarshalerWithContext(zctx)
	m.Decorate(sup.StyleSimple)
	return &LogReader{
		ctx:       ctx,
		marshaler: m,
		store:     store,
		cursor:    leaf,
		stop:      stop,
	}
}

func (r *LogReader) Read() (*super.Value, error) {
	if r.cursor == ksuid.Nil {
		return nil, nil
	}
	_, commitObject, err := r.store.GetBytes(r.ctx, r.cursor)
	if err != nil {
		return nil, err
	}
	next := commitObject.Parent
	if next == r.stop {
		next = ksuid.Nil
	}
	r.cursor = next
	val, err := r.marshaler.Marshal(commitObject)
	return &val, err
}

package exec

import (
	"sync"

	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/sbuf"
	"github.com/segmentio/ksuid"
)

type DeleteQuery struct {
	*Query
	deletes *sync.Map
}

var _ runtime.DeleteQuery = (*DeleteQuery)(nil)

func NewDeleteQuery(rctx *runtime.Context, puller sbuf.Puller, deletes *sync.Map) *DeleteQuery {
	return &DeleteQuery{
		Query:   NewQuery(rctx, puller, nil),
		deletes: deletes,
	}
}

func (d *DeleteQuery) DeletionSet() []ksuid.KSUID {
	var ids []ksuid.KSUID
	if d.deletes != nil {
		d.deletes.Range(func(key, value any) bool {
			ids = append(ids, key.(ksuid.KSUID))
			return true
		})
	}
	return ids
}

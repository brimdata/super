package fjsonio

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/bytedance/sonic/ast"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/fjsonio/jsonvec"
	"github.com/brimdata/super/vector"
)

var VecBatchSize uint32 = 10 * 1024

type VectorReader struct {
	sctx     *super.Context
	ctx      context.Context
	stream   *stream
	pushdown sbuf.Pushdown

	hasClosed atomic.Bool
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, p sbuf.Pushdown, concurrentReaders int) *VectorReader {
	return &VectorReader{
		sctx:     sctx,
		ctx:      ctx,
		stream:   newStream(ctx, r, concurrentReaders),
		pushdown: p,
	}
}

func (v *VectorReader) Pull(done bool) (vector.Any, error) {
	return v.ConcurrentPull(done, 0)
}

func (v *VectorReader) ConcurrentPull(done bool, _ int) (vector.Any, error) {
	if done {
		return nil, v.close()
	}
	table, err := v.stream.next()
	if table == nil || err != nil {
		v.close()
		return nil, err
	}
	// XXX Support projections.
	builder := jsonvec.NewBuilder()
	for i := range table.Len() {
		if err := ast.Preorder(byteconv.UnsafeString(table.Bytes(i)), builder, nil); err != nil {
			v.close()
			return nil, err
		}
	}
	return jsonvec.Materialize(v.sctx, builder), nil
}

func (v *VectorReader) close() error {
	if v.hasClosed.CompareAndSwap(false, true) {
		return nil
	}
	return v.stream.close()
}

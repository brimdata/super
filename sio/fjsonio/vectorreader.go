package fjsonio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/bytedance/sonic/ast"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/fjsonio/jsonvec"
	"github.com/brimdata/super/vector"
)

var VecBatchSize uint32 = 10 * 1024

type VectorReader struct {
	sctx       *super.Context
	ctx        context.Context
	stream     *stream
	pushdown   sbuf.Pushdown
	projection field.List

	hasClosed atomic.Bool
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, p sbuf.Pushdown, concurrentReaders int) *VectorReader {
	var fields field.List
	if p != nil {
		fields = p.Projection().Paths()
	}
	return &VectorReader{
		sctx:       sctx,
		ctx:        ctx,
		stream:     newStream(ctx, r, concurrentReaders),
		projection: fields,
	}
}

func (v *VectorReader) Pull(done bool) (vector.Any, error) {
	return v.ConcurrentPull(done, 0)
}

func (v *VectorReader) ConcurrentPull(done bool, _ int) (vector.Any, error) {
	if done {
		return nil, v.close()
	}
	table, start, err := v.stream.next()
	if table == nil || err != nil {
		v.close()
		return nil, err
	}
	builder := v.newBuilder()
	for i := range table.Len() {
		if err := ast.Preorder(byteconv.UnsafeString(table.Bytes(i)), builder, nil); err != nil {
			err = preorderErr(i, start, table, err)
			bytesTablePool.Put(table)
			v.close()
			return nil, err
		}
	}
	bytesTablePool.Put(table)
	return jsonvec.Materialize(v.sctx, builder), nil
}

func preorderErr(idx uint32, start int, table *vector.BytesTable, err error) error {
	b := table.RawBytes()
	off := int(table.RawOffsets()[idx])
	lineNum := start + bytes.Count(b[:off], []byte{'\n'})
	if i := firstNonWhitespaceCharacter(b[off:]); i != -1 {
		lineNum += bytes.Count(b[off:off+i], []byte{'\n'})
	}
	return fmt.Errorf("line %d: %w", lineNum, err)
}

func (v *VectorReader) newBuilder() jsonvec.Builder {
	if v.projection != nil {
		return jsonvec.NewProjectionBuilder(v.projection)
	}
	return jsonvec.NewBuilder()
}

func (v *VectorReader) close() error {
	if v.hasClosed.CompareAndSwap(false, true) {
		return v.stream.close()
	}
	return nil
}

package index

import (
	"context"
	"strings"
	"testing"

	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	r := NewTypeIndex(zng.TypeInt64)
	ref := Reference{Index: r, SegmentID: ksuid.New()}
	w := testWriter(t, ref)
	err := zio.Copy(w, babbleReader(t))
	require.NoError(t, err, "copy error")
	require.NoError(t, w.Close())
}

func TestWriterWriteAfterClose(t *testing.T) {
	r := NewTypeIndex(zng.TypeInt64)
	ref := Reference{Index: r, SegmentID: ksuid.New()}
	w := testWriter(t, ref)
	require.NoError(t, w.Close())
	err := w.Write(nil)
	assert.EqualError(t, err, "index writer closed")
	err = w.Write(nil)
	assert.EqualError(t, err, "index writer closed")
}

func TestWriterError(t *testing.T) {
	const r1 = `{ts:1970-01-01T00:00:01Z,id:"id1"}`
	const r2 = "{ts:1970-01-01T00:00:02Z,id:2}"
	ref := Reference{Index: NewFieldIndex("id"), SegmentID: ksuid.New()}
	w := testWriter(t, ref)
	zctx := zson.NewContext()
	arr1, err := zbuf.ReadAll(zson.NewReader(strings.NewReader(r1), zctx))
	require.NoError(t, err)
	arr2, err := zbuf.ReadAll(zson.NewReader(strings.NewReader(r2), zctx))
	require.NoError(t, err)
	require.NoError(t, zio.Copy(w, arr1.NewReader()))
	require.NoError(t, zio.Copy(w, arr2.NewReader()))

	err = w.Close()
	assert.EqualError(t, err, `key type changed from "{key:int64}" to "{key:string}"`)

	// if an on close, the writer should have removed the index
	assert.NoFileExists(t, w.URI.Filepath())
}

func testWriter(t *testing.T, ref Reference) *Writer {
	path := storage.MustParseURI(t.TempDir())
	w, err := NewWriter(context.Background(), storage.NewLocalEngine(), path, &ref)
	require.NoError(t, err)
	return w
}

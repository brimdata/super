package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superdb/super"
	"github.com/superdb/super/db/data"
	"github.com/superdb/super/order"
	"github.com/superdb/super/pkg/field"
	"github.com/superdb/super/pkg/storage"
	"github.com/superdb/super/sbuf"
	"github.com/superdb/super/sio/bsupio"
	"github.com/superdb/super/sup"
)

func TestDataReaderWriterVector(t *testing.T) {
	engine := storage.NewLocalEngine()
	tmp := storage.MustParseURI(t.TempDir())
	object := data.NewObject()
	ctx := t.Context()
	w, err := object.NewWriter(ctx, engine, tmp, order.NewSortKey(order.Asc, field.Path{"a"}))
	require.NoError(t, err)
	sctx := super.NewContext()
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:1,b:4}")))
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:2,b:5}")))
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:3,b:6}")))
	require.NoError(t, w.Close(ctx))
	// Read back the BSUP file and make sure it's the same.
	get, err := engine.Get(ctx, object.VectorURI(tmp))
	require.NoError(t, err)
	p, err := bsupio.NewReader(t.Context(), super.NewContext(), get, nil, 1)
	require.NoError(t, err)
	defer p.Pull(true)
	reader := sbuf.PullerReader(sbuf.NewMaterializer(p))
	v, err := reader.Read()
	require.NoError(t, err)
	assert.Equal(t, sup.String(v), "{a:1,b:4}")
	v, err = reader.Read()
	require.NoError(t, err)
	assert.Equal(t, sup.String(v), "{a:2,b:5}")
	v, err = reader.Read()
	require.NoError(t, err)
	assert.Equal(t, sup.String(v), "{a:3,b:6}")
	require.NoError(t, get.Close())
	require.NoError(t, data.DeleteVector(ctx, engine, tmp, object.ID))
	exists, err := engine.Exists(ctx, data.VectorURI(tmp, object.ID))
	require.NoError(t, err)
	assert.Equal(t, exists, false)
}

package data_test

import (
	"context"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataReaderWriterVector(t *testing.T) {
	engine := storage.NewLocalEngine()
	tmp := storage.MustParseURI(t.TempDir())
	object := data.NewObject()
	ctx := context.Background()
	w, err := object.NewWriter(ctx, engine, tmp, order.NewSortKey(order.Asc, field.Path{"a"}), 1000)
	require.NoError(t, err)
	sctx := super.NewContext()
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:1,b:4}")))
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:2,b:5}")))
	require.NoError(t, w.Write(sup.MustParseValue(sctx, "{a:3,b:6}")))
	require.NoError(t, w.Close(ctx))
	require.NoError(t, data.CreateVector(ctx, engine, tmp, object.ID))
	// Read back the CSUP file and make sure it's the same.
	get, err := engine.Get(ctx, object.VectorURI(tmp))
	require.NoError(t, err)
	reader, err := csupio.NewReader(super.NewContext(), get, nil)
	require.NoError(t, err)
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

package queryio_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/api/queryio"
	"github.com/brimdata/super/zson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZJSONWriter(t *testing.T) {
	const record = `{x:1}`
	const expected = `
{"type":"QueryChannelSet","value":{"channel":"main"}}
{"type":{"kind":"record","id":30,"fields":[{"name":"x","type":{"kind":"primitive","name":"int64"}}]},"value":["1"]}
{"type":"QueryChannelEnd","value":{"channel":"main"}}
{"type":"QueryError","value":{"error":"test.err"}}
`
	var buf bytes.Buffer
	w := queryio.NewZJSONWriter(&buf)
	err := w.WriteControl(api.QueryChannelSet{Channel: "main"})
	require.NoError(t, err)
	err = w.Write(zson.MustParseValue(super.NewContext(), record))
	require.NoError(t, err)
	err = w.WriteControl(api.QueryChannelEnd{Channel: "main"})
	require.NoError(t, err)
	err = w.WriteControl(api.QueryError{Error: "test.err"})
	require.NoError(t, err)
	assert.Equal(t, expected, "\n"+buf.String())
}

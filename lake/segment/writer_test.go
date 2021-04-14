package segment

import (
	"context"
	"strings"
	"testing"

	"github.com/brimdata/zed/lake/index"
	"github.com/brimdata/zed/pkg/iosrc"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zson"
	"github.com/stretchr/testify/require"
)

/* NOT YET
func TestWriterIndex(t *testing.T) {
	const data = `
{ts:1970-01-01T00:00:05Z,v:100}
{ts:1970-01-01T00:00:04Z,v:101}
{ts:1970-01-01T00:00:03Z,v:104}
{ts:1970-01-01T00:00:02Z,v:109}
{ts:1970-01-01T00:00:01Z,v:100}
`
	def := index.MustNewDefinition(index.NewTypeRule(zng.TypeInt64))
	chunk := testWriteWithDef(t, data, def)
	reader, err := index.Find(context.Background(), zson.NewContext(), chunk.ZarDir(), def.ID, "100")
	require.NoError(t, err)
	recs, err := zbuf.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Len(t, recs, 1)
	v, err := recs[0].AccessInt("count")
	require.NoError(t, err)
	require.EqualValues(t, 2, v)
}
*/

/* NOT YET
func TestWriterSkipsInputPath(t *testing.T) {
	const data = `{ts:1970-01-01T00:00:05Z,v:100,s:"test"}`
	sdef := index.MustNewDefinition(index.NewFieldRule("s"))
	inputdef := index.MustNewDefinition(index.NewTypeRule(zng.TypeInt64))
	inputdef.Input = "input_path"
	zctx := zson.NewContext()
	chunk := testWriteWithDef(t, data, sdef, inputdef)
	//reader, err := index.Find(context.Background(), zctx, chunk.ZarDir(), sdef.ID, "test")
	//require.NoError(t, err)
	recs, err := zbuf.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Len(t, recs, 1)
	_, err = index.Find(context.Background(), zctx, chunk.ZarDir(), inputdef.ID, "100")
	assert.Truef(t, zqe.IsNotFound(err), "expected err to be zqe.IsNotFound, got: %v", err)
}
*/

func testWriteWithDef(t *testing.T, input string, defs ...*index.Definition) *Reference {
	dir := iosrc.MustParseURI(t.TempDir())
	ref := New()
	w, err := ref.NewWriter(context.Background(), dir, WriterOpts{Order: zbuf.OrderDesc, Definitions: defs})
	require.NoError(t, err)
	require.NoError(t, zbuf.Copy(w, zson.NewReader(strings.NewReader(input), zson.NewContext())))
	require.NoError(t, w.Close(context.Background()))
	return w.Segment()
}

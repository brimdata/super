package archive

import (
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/brimsec/zq/microindex"
	"github.com/brimsec/zq/pkg/iosrc"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/pkg/promtest"
	"github.com/brimsec/zq/pkg/test"
	"github.com/brimsec/zq/ppl/archive/chunk"
	"github.com/brimsec/zq/ppl/archive/immcache"
	"github.com/brimsec/zq/ppl/archive/index"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/detector"
	"github.com/brimsec/zq/zio/tzngio"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const babble = "../../ztests/suite/data/babble.tzng"

func createArchiveSpace(t *testing.T, datapath string, srcfile string, co *CreateOptions) {
	ark, err := CreateOrOpenArchive(datapath, co, nil)
	require.NoError(t, err)

	importTestFile(t, ark, srcfile)
}

func importTestFile(t *testing.T, ark *Archive, srcfile string) {
	zctx := resolver.NewContext()
	reader, err := detector.OpenFile(zctx, srcfile, zio.ReaderOpts{})
	require.NoError(t, err)
	defer reader.Close()

	err = Import(context.Background(), ark, zctx, reader)
	require.NoError(t, err)
}

func indexArchiveSpace(t *testing.T, datapath string, ruledef string) {
	rule, err := index.NewRule(ruledef)
	require.NoError(t, err)

	ark, err := OpenArchive(datapath, nil)
	require.NoError(t, err)

	err = ApplyRules(context.Background(), ark, nil, rule)
	require.NoError(t, err)
}

func indexQuery(t *testing.T, ark *Archive, patterns []string, opts ...FindOption) string {
	q, err := index.ParseQuery("", patterns)
	require.NoError(t, err)
	rc, err := FindReadCloser(context.Background(), resolver.NewContext(), ark, q, opts...)
	require.NoError(t, err)
	defer rc.Close()

	var buf bytes.Buffer
	w := tzngio.NewWriter(zio.NopCloser(&buf))
	require.NoError(t, zbuf.Copy(w, rc))

	return buf.String()
}

func TestMetadataCache(t *testing.T) {
	datapath := t.TempDir()
	createArchiveSpace(t, datapath, babble, nil)
	reg := prometheus.NewRegistry()
	icache, err := immcache.NewLocalCache(128, reg)
	require.NoError(t, err)

	ark, err := OpenArchive(datapath, &OpenOptions{
		ImmutableCache: icache,
	})
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		count, err := RecordCount(context.Background(), ark)
		require.NoError(t, err)
		assert.EqualValues(t, 1000, count)
	}

	kind := prometheus.Labels{"kind": "metadata"}
	misses := promtest.CounterValue(t, reg, "archive_cache_misses_total", kind)
	hits := promtest.CounterValue(t, reg, "archive_cache_hits_total", kind)

	assert.EqualValues(t, 2, misses)
	assert.EqualValues(t, 6, hits)
}

func TestSeekIndex(t *testing.T) {
	datapath := t.TempDir()

	orig := ImportStreamRecordsMax
	ImportStreamRecordsMax = 1
	defer func() {
		ImportStreamRecordsMax = orig
	}()
	createArchiveSpace(t, datapath, babble, nil)
	_, err := OpenArchive(datapath, &OpenOptions{})
	require.NoError(t, err)

	first1 := nano.Ts(1587513592062544400)
	var idxURI iosrc.URI
	err = filepath.Walk(datapath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if k, id, ok := chunk.FileMatch(fi.Name()); ok && k == chunk.FileKindMetadata {
			uri, err := iosrc.ParseURI(p)
			if err != nil {
				return err
			}
			uri.Path = path.Dir(uri.Path)
			chunk, err := chunk.Open(context.Background(), uri, id, zbuf.OrderDesc)
			if err != nil {
				return err
			}
			if chunk.First == first1 {
				idxURI = chunk.SeekIndexPath()
			}
		}
		return nil
	})
	require.NoError(t, err)
	finder, err := microindex.NewFinder(context.Background(), resolver.NewContext(), idxURI)
	require.NoError(t, err)
	keys, err := finder.ParseKeys("1587508851")
	require.NoError(t, err)
	rec, err := finder.ClosestLTE(keys)
	require.NoError(t, err)
	require.NoError(t, finder.Close())

	var buf bytes.Buffer
	w := tzngio.NewWriter(zio.NopCloser(&buf))
	require.NoError(t, w.Write(rec))

	exp := `
#0:record[ts:time,offset:int64]
0:[1587508850.06466032;23795;]
`
	require.Equal(t, test.Trim(exp), buf.String())
}

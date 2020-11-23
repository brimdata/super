package archive

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/ppl/archive/chunk"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio/tzngio"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kid(s string) ksuid.KSUID {
	var b [20]byte
	copy(b[:], s)
	k, _ := ksuid.FromBytes(b[:])
	return k
}

func importTzng(t *testing.T, ark *Archive, s string) {
	zctx := resolver.NewContext()
	reader := tzngio.NewReader(strings.NewReader(s), zctx)
	err := Import(context.Background(), ark, zctx, reader)
	require.NoError(t, err)
}

func TestAlignChunksToSpans(t *testing.T) {
	cases := []struct {
		chunks []chunk.Chunk
		filter nano.Span
		order  zbuf.Order
		exp    []SpanInfo
	}{
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 0},
				{Id: kid("b"), First: 1, Last: 1},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 0}}},
				{Span: nano.Span{Ts: 1, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("b"), First: 1, Last: 1}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 1},
				{Id: kid("b"), First: 1, Last: 2},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 1}}},
				{Span: nano.Span{Ts: 1, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 1}, {Id: kid("b"), First: 1, Last: 2}}},
				{Span: nano.Span{Ts: 2, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("b"), First: 1, Last: 2}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 3},
				{Id: kid("b"), First: 1, Last: 2},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 3}}},
				{Span: nano.Span{Ts: 1, Dur: 2}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 3}, {Id: kid("b"), First: 1, Last: 2}}},
				{Span: nano.Span{Ts: 3, Dur: 1}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 3}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 3},
				{Id: kid("b"), First: 1, Last: 2},
			},
			filter: nano.Span{Ts: 1, Dur: 2},
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 1, Dur: 2}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 0, Last: 3}, {Id: kid("b"), First: 1, Last: 2}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 9, Last: 7},
				{Id: kid("b"), First: 5, Last: 3},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderDesc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 7, Dur: 3}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 9, Last: 7}}},
				{Span: nano.Span{Ts: 3, Dur: 3}, Chunks: []chunk.Chunk{{Id: kid("b"), First: 5, Last: 3}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 9, Last: 5},
				{Id: kid("b"), First: 7, Last: 3},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderDesc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 8, Dur: 2}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 9, Last: 5}}},
				{Span: nano.Span{Ts: 5, Dur: 3}, Chunks: []chunk.Chunk{{Id: kid("a"), First: 9, Last: 5}, {Id: kid("b"), First: 7, Last: 3}}},
				{Span: nano.Span{Ts: 3, Dur: 2}, Chunks: []chunk.Chunk{{Id: kid("b"), First: 7, Last: 3}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("b"), First: 0, Last: 0},
				{Id: kid("a"), First: 0, Last: 0},
				{Id: kid("d"), First: 0, Last: 0},
				{Id: kid("c"), First: 0, Last: 0},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 0},
					{Id: kid("b"), First: 0, Last: 0},
					{Id: kid("c"), First: 0, Last: 0},
					{Id: kid("d"), First: 0, Last: 0}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 5},
				{Id: kid("b"), First: 1, Last: 8},
				{Id: kid("c"), First: 6, Last: 6},
				{Id: kid("d"), First: 7, Last: 10},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 5}}},
				{Span: nano.Span{Ts: 1, Dur: 5}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 5},
					{Id: kid("b"), First: 1, Last: 8}}},
				{Span: nano.Span{Ts: 6, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("b"), First: 1, Last: 8},
					{Id: kid("c"), First: 6, Last: 6}}},
				{Span: nano.Span{Ts: 7, Dur: 2}, Chunks: []chunk.Chunk{
					{Id: kid("b"), First: 1, Last: 8},
					{Id: kid("d"), First: 7, Last: 10}}},
				{Span: nano.Span{Ts: 9, Dur: 2}, Chunks: []chunk.Chunk{
					{Id: kid("d"), First: 7, Last: 10}}},
			},
		},
		{
			chunks: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 10},
				{Id: kid("b"), First: 1, Last: 10},
				{Id: kid("c"), First: 2, Last: 10},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 10}}},
				{Span: nano.Span{Ts: 1, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 10},
					{Id: kid("b"), First: 1, Last: 10}}},
				{Span: nano.Span{Ts: 2, Dur: 9}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 10},
					{Id: kid("b"), First: 1, Last: 10},
					{Id: kid("c"), First: 2, Last: 10}}},
			},
		},
		{
			chunks: nil,
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp:    nil,
		},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, c.exp, alignChunksToSpans(c.chunks, c.order, c.filter))
		})
	}
}

func TestOverlapWalking(t *testing.T) {
	datapath, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(datapath)

	ark, err := CreateOrOpenArchive(datapath, &CreateOptions{}, nil)
	require.NoError(t, err)

	data1 := `
#0:record[ts:time,v:int64]
0:[0;0;]
0:[.000000005;5;]
`
	data2 := `
#0:record[ts:time,v:int64]
0:[.000000010;10;]
0:[.000000020;20;]
`
	data3 := `
#0:record[ts:time,v:int64]
0:[.000000015;15;]
0:[.000000025;25;]
`
	dataChunkSpans := []nano.Span{{Ts: 15, Dur: 11}, {Ts: 10, Dur: 11}, {Ts: 0, Dur: 6}}
	importTzng(t, ark, data2)
	importTzng(t, ark, data1)
	importTzng(t, ark, data3)

	{
		var chunks []chunk.Chunk
		err = tsDirVisit(context.Background(), ark, nano.MaxSpan, func(tsd tsDir, c []chunk.Chunk) error {
			chunks = append(chunks, c...)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, chunks, 3)
		chunk.Sort(ark.DataOrder, chunks)
		var spans []nano.Span
		for _, c := range chunks {
			spans = append(spans, c.Span())
		}
		require.Equal(t, dataChunkSpans, spans)
	}
	{
		var chunks []chunk.Chunk
		err = Walk(context.Background(), ark, func(c chunk.Chunk) error {
			chunks = append(chunks, c)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, chunks, 3)
		var spans []nano.Span
		for _, c := range chunks {
			spans = append(spans, c.Span())
		}
		require.Equal(t, dataChunkSpans, spans)
	}
	{
		var chunks []chunk.Chunk
		err = tsDirVisit(context.Background(), ark, nano.Span{Ts: 12, Dur: 20}, func(tsd tsDir, c []chunk.Chunk) error {
			chunks = append(chunks, c...)
			return nil
		})
		require.NoError(t, err)
		assert.Len(t, chunks, 2)
		chunk.Sort(ark.DataOrder, chunks)
		var spans []nano.Span
		for _, c := range chunks {
			spans = append(spans, c.Span())
		}
		assert.Equal(t, []nano.Span{{Ts: 15, Dur: 11}, {Ts: 10, Dur: 11}}, spans)
	}
	{
		type sispan struct {
			si         nano.Span
			chunkSpans []nano.Span
		}
		var sispans []sispan
		err = SpanWalk(context.Background(), ark, nano.Span{Ts: 12, Dur: 10}, func(si SpanInfo) error {
			var chunkSpans []nano.Span
			for _, c := range si.Chunks {
				chunkSpans = append(chunkSpans, c.Span())
			}
			sispans = append(sispans, sispan{si: si.Span, chunkSpans: chunkSpans})
			return nil
		})
		require.NoError(t, err)
		assert.Len(t, sispans, 2)
		exp := []sispan{
			{si: nano.Span{Ts: 15, Dur: 7}, chunkSpans: []nano.Span{{Ts: 15, Dur: 11}, {Ts: 10, Dur: 11}}},
			{si: nano.Span{Ts: 12, Dur: 3}, chunkSpans: []nano.Span{{Ts: 10, Dur: 11}}},
		}
		assert.Equal(t, exp, sispans)
	}
}

func TestMergeLargestChunkSpanInfos(t *testing.T) {
	cases := []struct {
		in  []SpanInfo
		exp []SpanInfo
	}{
		{
			in: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 1, RecordCount: 10}}},
				{Span: nano.Span{Ts: 1, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 1, RecordCount: 10},
					{Id: kid("b"), First: 1, Last: 1, RecordCount: 5},
				}},
			},
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 2}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 1, RecordCount: 10},
					{Id: kid("b"), First: 1, Last: 1, RecordCount: 5},
				}},
			},
		},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, c.exp, mergeLargestChunkSpanInfos(c.in, zbuf.OrderAsc))
		})
	}
}

func TestMergeChunksToSpans(t *testing.T) {
	cases := []struct {
		in     []chunk.Chunk
		filter nano.Span
		order  zbuf.Order
		exp    []SpanInfo
	}{
		{
			in: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 3, RecordCount: 10},
				{Id: kid("b"), First: 1, Last: 3, RecordCount: 20},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 3, RecordCount: 10}}},
				{Span: nano.Span{Ts: 1, Dur: 3}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 3, RecordCount: 10},
					{Id: kid("b"), First: 1, Last: 3, RecordCount: 20}}},
			},
		},
		{
			in: []chunk.Chunk{
				{Id: kid("a"), First: 0, Last: 3, RecordCount: 20},
				{Id: kid("b"), First: 2, Last: 5, RecordCount: 10},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 4}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 3, RecordCount: 20},
					{Id: kid("b"), First: 2, Last: 5, RecordCount: 10}}},
				{Span: nano.Span{Ts: 4, Dur: 2}, Chunks: []chunk.Chunk{
					{Id: kid("b"), First: 2, Last: 5, RecordCount: 10}}},
			},
		},
		{
			in: []chunk.Chunk{
				{Id: kid("b"), First: 0, Last: 0, RecordCount: 10},
				{Id: kid("a"), First: 0, Last: 0, RecordCount: 10},
			},
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp: []SpanInfo{
				{Span: nano.Span{Ts: 0, Dur: 1}, Chunks: []chunk.Chunk{
					{Id: kid("a"), First: 0, Last: 0, RecordCount: 10},
					{Id: kid("b"), First: 0, Last: 0, RecordCount: 10}}},
			},
		},
		{
			in:     nil,
			filter: nano.MaxSpan,
			order:  zbuf.OrderAsc,
			exp:    nil,
		},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, c.exp, mergeChunksToSpans(c.in, c.order, c.filter))
		})
	}
}

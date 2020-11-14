package merge_test

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/brimsec/zq/field"
	"github.com/brimsec/zq/pkg/test"
	"github.com/brimsec/zq/proc"
	"github.com/brimsec/zq/proc/merge"
	"github.com/brimsec/zq/proc/proctest"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/tzngio"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var omTestInputs = []string{
	`
#0:record[v:int32,ts:time]
0:[10;1;]
0:[20;2;]
0:[30;3;]`,
	`
#0:record[v:int32,ts:time]
0:[15;4;]
0:[25;5;]
0:[35;6;]`,
}

var omTestInputRev = []string{
	`
#0:record[v:int32,ts:time]
0:[30;3;]
0:[20;2;]
0:[10;1;]
`,
	`
#0:record[v:int32,ts:time]
0:[35;6;]
0:[25;5;]
0:[15;4;]
`,
}

func TestParallelOrder(t *testing.T) {
	cases := []struct {
		field    string
		reversed bool
		inputs   []string
		exp      string
	}{
		{
			field:    "ts",
			reversed: false,
			inputs:   omTestInputs,
			exp: `
#0:record[v:int32,ts:time]
0:[10;1;]
0:[20;2;]
0:[30;3;]
0:[15;4;]
0:[25;5;]
0:[35;6;]
`,
		},
		{
			field:    "v",
			reversed: false,
			inputs:   omTestInputs,
			exp: `
#0:record[v:int32,ts:time]
0:[10;1;]
0:[15;4;]
0:[20;2;]
0:[25;5;]
0:[30;3;]
0:[35;6;]
`,
		},
		{
			field:    "ts",
			reversed: true,
			inputs:   omTestInputRev,
			exp: `
#0:record[v:int32,ts:time]
0:[35;6;]
0:[25;5;]
0:[15;4;]
0:[30;3;]
0:[20;2;]
0:[10;1;]
`,
		},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			zctx := resolver.NewContext()
			pctx := &proc.Context{Context: context.Background(), TypeContext: zctx}
			var parents []proc.Interface
			for _, s := range c.inputs {
				r := tzngio.NewReader(bytes.NewReader([]byte(s)), zctx)
				parents = append(parents, &proctest.RecordPuller{R: r})
			}
			cmp := zbuf.NewCompareFn(field.New(c.field), c.reversed)
			om := merge.New(pctx.Context, parents, cmp)

			var sb strings.Builder
			err := zbuf.CopyPuller(tzngio.NewWriter(zio.NopCloser(&sb)), om)
			require.NoError(t, err)
			assert.Equal(t, test.Trim(c.exp), sb.String())
		})
	}
}

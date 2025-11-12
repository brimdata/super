package merge_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/merge"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/supio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var omTestInputs = []string{
	`
{v:10,ts:1970-01-01T00:00:01Z}
{v:20,ts:1970-01-01T00:00:02Z}
{v:30,ts:1970-01-01T00:00:03Z}
`,
	`
{v:15,ts:1970-01-01T00:00:04Z}
{v:25,ts:1970-01-01T00:00:05Z}
{v:35,ts:1970-01-01T00:00:06Z}
`,
}

var omTestInputRev = []string{
	`
{v:30,ts:1970-01-01T00:00:03Z}
{v:20,ts:1970-01-01T00:00:02Z}
{v:10,ts:1970-01-01T00:00:01Z}
`,
	`
{v:35,ts:1970-01-01T00:00:06Z}
{v:25,ts:1970-01-01T00:00:05Z}
{v:15,ts:1970-01-01T00:00:04Z}
`,
}

func TestParallelOrder(t *testing.T) {
	cases := []struct {
		field  string
		order  order.Which
		inputs []string
		exp    string
	}{
		{
			field:  "ts",
			order:  order.Asc,
			inputs: omTestInputs,
			exp: `
{v:10,ts:1970-01-01T00:00:01Z}
{v:20,ts:1970-01-01T00:00:02Z}
{v:30,ts:1970-01-01T00:00:03Z}
{v:15,ts:1970-01-01T00:00:04Z}
{v:25,ts:1970-01-01T00:00:05Z}
{v:35,ts:1970-01-01T00:00:06Z}
`,
		},
		{

			field:  "v",
			order:  order.Asc,
			inputs: omTestInputs,
			exp: `
{v:10,ts:1970-01-01T00:00:01Z}
{v:15,ts:1970-01-01T00:00:04Z}
{v:20,ts:1970-01-01T00:00:02Z}
{v:25,ts:1970-01-01T00:00:05Z}
{v:30,ts:1970-01-01T00:00:03Z}
{v:35,ts:1970-01-01T00:00:06Z}
`,
		},
		{
			field:  "ts",
			order:  order.Desc,
			inputs: omTestInputRev,
			exp: `
{v:35,ts:1970-01-01T00:00:06Z}
{v:25,ts:1970-01-01T00:00:05Z}
{v:15,ts:1970-01-01T00:00:04Z}
{v:30,ts:1970-01-01T00:00:03Z}
{v:20,ts:1970-01-01T00:00:02Z}
{v:10,ts:1970-01-01T00:00:01Z}
`,
		},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			sctx := super.NewContext()
			var parents []sbuf.Puller
			for _, input := range c.inputs {
				r := supio.NewReader(sctx, strings.NewReader(input))
				parents = append(parents, sbuf.NewPuller(r))
			}
			sortExpr := expr.NewSortExpr(expr.NewDottedExpr(sctx, field.Path{c.field}), c.order, order.NullsLast)
			cmp := expr.NewComparator(sortExpr).Compare
			om := merge.New(t.Context(), parents, cmp)

			var sb strings.Builder
			err := sbuf.CopyPuller(supio.NewWriter(sio.NopCloser(&sb), supio.WriterOpts{}), om)
			require.NoError(t, err)
			assert.Equal(t, c.exp, "\n"+sb.String())
		})
	}
}

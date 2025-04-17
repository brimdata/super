package expr

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/sup"
)

func BenchmarkSort(b *testing.B) {
	cases := []struct {
		typ   super.Type
		bytes func() []byte
	}{
		{super.TypeInt64, func() []byte { return super.EncodeInt(int64(rand.Uint64())) }},
		{super.TypeUint64, func() []byte { return super.EncodeUint(rand.Uint64()) }},
		{super.TypeString, func() []byte { return strconv.AppendUint(nil, rand.Uint64(), 16) }},
		{super.TypeDuration, func() []byte { return super.EncodeInt(int64(rand.Uint64())) }},
		{super.TypeTime, func() []byte { return super.EncodeInt(int64(rand.Uint64())) }},
	}
	for _, c := range cases {
		b.Run(sup.FormatType(c.typ), func(b *testing.B) {
			cmp := NewComparator(false, SortExpr{&This{}, order.Asc})
			vals := make([]super.Value, 1048576)
			for b.Loop() {
				b.StopTimer()
				for i := range vals {
					vals[i] = super.NewValue(c.typ, c.bytes())
				}
				b.StartTimer()
				cmp.SortStable(vals)
			}
		})
	}
}

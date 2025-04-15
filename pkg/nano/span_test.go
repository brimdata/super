package nano_test

import (
	"testing"

	"github.com/brimdata/super/pkg/nano"
	"github.com/stretchr/testify/assert"
)

func TestSubSpan(t *testing.T) {
	t.Parallel()
	const n = 4
	s := nano.Span{Ts: 0, Dur: 5*n + 1}

	var ss []nano.Span
	for i := range n {
		ss = append(ss, s.SubSpan(i, n))
	}

	// Start of the first subspan should equal start of the span.
	assert.Exactly(t, s.Ts, ss[0].Ts)

	// End of the last subspan should equal end of the span.
	assert.Exactly(t, s.End(), ss[n-1].End())

	for i := range ss[:n-1] {
		// End of a subspan should equal the start of the next subspan.
		assert.Exactly(t, ss[i+1].Ts, ss[i].End())
	}

}

func TestPartition(t *testing.T) {
	//XXX this breaks with tree alignment changes
	t.Skip()
	t.Parallel()
	const n = 4
	s := nano.Span{Ts: 0, Dur: n + 1}

	cases := []struct {
		ts    nano.Ts
		index int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 3},
		{4, 3},
	}
	for _, c := range cases {
		assert.Exactly(t, c.index, s.Partition(c.ts, n), "ts %v", c.ts)
	}
}

func TestOverlapsOrAdjacent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a        nano.Span
		b        nano.Span
		expected bool
	}{
		{
			nano.Span{Ts: 0, Dur: 1},
			nano.Span{Ts: 1, Dur: 1},
			true,
		},
		{
			nano.Span{Ts: 1, Dur: 1},
			nano.Span{Ts: 0, Dur: 1},
			true,
		},
		{
			nano.Span{Ts: 0, Dur: 1},
			nano.Span{Ts: 2, Dur: 1},
			false,
		},
		{
			nano.Span{Ts: 0, Dur: 2},
			nano.Span{Ts: 1, Dur: 1},
			true,
		},
	}

	for _, c := range cases {
		assert.Exactly(t, c.expected, c.a.OverlapsOrAdjacent(c.b), "%v OverlapsOrAdjacent %v", c.a, c.b)
	}
}

func TestOverlaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a        nano.Span
		b        nano.Span
		expected bool
	}{
		{
			nano.Span{Ts: 0, Dur: 1},
			nano.Span{Ts: 1, Dur: 1},
			false,
		},
		{
			nano.Span{Ts: 1, Dur: 1},
			nano.Span{Ts: 0, Dur: 1},
			false,
		},
		{
			nano.Span{Ts: 0, Dur: 2},
			nano.Span{Ts: 1, Dur: 1},
			true,
		},
	}

	for _, c := range cases {
		assert.Exactly(t, c.expected, c.a.Overlaps(c.b), "%v Overlaps %v", c.a, c.b)
	}
}

func TestSubtract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		a        nano.Span
		b        nano.Span
		expected []nano.Span
	}{
		{
			"split",
			nano.Span{Ts: 0, Dur: 10},
			nano.Span{Ts: 4, Dur: 2},
			[]nano.Span{
				{Ts: 0, Dur: 4},
				{Ts: 6, Dur: 4},
			},
		},
		{
			"subtract_front",
			nano.Span{Ts: 2, Dur: 8},
			nano.Span{Ts: 0, Dur: 5},
			[]nano.Span{
				{Ts: 5, Dur: 5},
			},
		},
		{
			"subtract_back",
			nano.Span{Ts: 0, Dur: 10},
			nano.Span{Ts: 5, Dur: 15},
			[]nano.Span{
				{Ts: 0, Dur: 5},
			},
		},
		{
			"subtract_all",
			nano.Span{Ts: 0, Dur: 10},
			nano.Span{Ts: 0, Dur: 10},
			[]nano.Span{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Exactly(t, c.expected, c.a.Subtract(c.b), "%v Subract %v", c.a, c.b)
		})
	}
}

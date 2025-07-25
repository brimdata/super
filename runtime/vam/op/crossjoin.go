package op

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type crossJoin struct {
	rctx       *runtime.Context
	left       vector.Puller
	right      vector.Puller
	leftAlias  string
	rightAlias string

	inner          *bufPuller
	innerVecsIndex int
	outer          *bufPuller
	outerIndexes   [][]uint32
	outerVec       vector.Any
	outerVecSlot   uint32
	swap           bool
}

func NewCrossJoin(rctx *runtime.Context, left, right vector.Puller, leftAlias, rightAlias string) vector.Puller {
	return &crossJoin{
		rctx:       rctx,
		left:       left,
		right:      right,
		leftAlias:  leftAlias,
		rightAlias: rightAlias,
	}
}

func (c *crossJoin) Pull(done bool) (vector.Any, error) {
	if done {
		c.outer = nil
		_, err := c.left.Pull(true)
		if err == nil {
			_, err = c.right.Pull(true)
		}
		return nil, err
	}
	if c.outer == nil {
		var err error
		c.outer, c.inner, err = pullRace(c.rctx.Context, c.left, c.right)
		if err != nil {
			return nil, err
		}
		if c.outer.EOS {
			c.inner, c.outer = c.outer, c.inner
			c.swap = true
		}
		c.outerVec = nil
	}
	if c.outerVec == nil || c.outerVecSlot >= c.outerVec.Len() {
		var err error
		c.outerVec, err = c.outer.Pull(false)
		if c.outerVec == nil || err != nil {
			return nil, err
		}
		c.innerVecsIndex = 0
		c.outerVecSlot = 0
	}
	innerVec := c.inner.vecs[c.innerVecsIndex]
	outerVec := c.makeOuterVec(int(innerVec.Len()))
	if c.swap {
		outerVec, innerVec = innerVec, outerVec
	}
	c.innerVecsIndex++
	if c.innerVecsIndex >= len(c.inner.vecs) {
		c.innerVecsIndex = 0
		c.outerVecSlot++
	}
	return vector.Apply(false, c.makeResult, outerVec, innerVec), nil
}

func (c *crossJoin) makeOuterVec(length int) vector.Any {
	if n := int(c.outerVec.Len()); n > len(c.outerIndexes) {
		c.outerIndexes = slices.Grow(c.outerIndexes[:0], n)[:n]
	}
	index := c.outerIndexes[c.outerVecSlot]
	if len(index) < length {
		index = slices.Grow(index[:0], length)[:length]
		for i := range index {
			index[i] = c.outerVecSlot
		}
		c.outerIndexes[c.outerVecSlot] = index
	}
	return vector.Pick(c.outerVec, index[:length])
}

func (c *crossJoin) makeResult(vecs ...vector.Any) vector.Any {
	if len(vecs) != 2 {
		panic(vecs)
	}
	left, right := vecs[0], vecs[1]
	typ := c.rctx.Sctx.MustLookupTypeRecord([]super.Field{
		super.NewField(c.leftAlias, left.Type()),
		super.NewField(c.rightAlias, right.Type()),
	})
	return vector.NewRecord(typ, []vector.Any{left, right}, left.Len(), bitvec.Zero)
}

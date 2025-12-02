package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Coalesce struct {
	tags        []uint32
	viewIndexes [][]uint32
	setslots    *vector.Bool
	setcnt      uint32
}

func (c *Coalesce) RipUnions() bool { return false }

func (c *Coalesce) Call(vecs ...vector.Any) vector.Any {
	args := underAll(vecs)
	c.tags = make([]uint32, args[0].Len())
	c.viewIndexes = make([][]uint32, len(args))
	c.setslots = vector.NewFalse(args[0].Len())
	c.setcnt = 0
	size := args[0].Len()
	for i, arg := range args {
		if c.setcnt >= size {
			c.viewIndexes = c.viewIndexes[:i]
			break
		}
		c.arg(arg, uint32(i))
	}
	n := len(c.viewIndexes)
	var nullcnt uint32
	if c.setcnt < size {
		// Set the nulls for all rows where nothing was set.
		for i := range c.setslots.Len() {
			if !c.setslots.IsSet(i) {
				c.tags[i] = uint32(n)
				nullcnt++
			}
		}
	}
	out := make([]vector.Any, n)
	for i := range n {
		out[i] = vector.Pick(vecs[i], c.viewIndexes[i])
	}
	if nullcnt > 0 {
		out = append(out, vector.NewConst(super.Null, nullcnt, bitvec.Zero))
	}
	return vector.NewDynamic(c.tags, out)
}

func (c *Coalesce) arg(vec vector.Any, tag uint32) {
	if _, ok := vec.(*vector.Error); !ok {
		c.checkNulls(vec, tag)
	}
}

func (c *Coalesce) checkNulls(vec vector.Any, tag uint32) {
	switch vec := vec.(type) {
	case *vector.View:
		if nulls := vector.NullsOf(vec.Any); !nulls.IsZero() {
			for i := range vec.Len() {
				if !nulls.IsSetDirect(vec.Index[i]) {
					c.setTag(i, tag)
				}
			}
			return
		}
	case *vector.Const:
		if val := vec.Value(); val.IsNull() {
			return
		}
	}
	c.setAll(vector.NullsOf(vec), tag)
}

func (c *Coalesce) setAll(nulls bitvec.Bits, tag uint32) {
	if !nulls.IsZero() {
		for i := range nulls.Len() {
			if !nulls.IsSetDirect(i) {
				c.setTag(i, tag)
			}
		}
	} else {
		for slot := range len(c.tags) {
			c.setTag(uint32(slot), tag)
		}
	}
}

// inline
func (c *Coalesce) setTag(slot, tag uint32) {
	if !c.setslots.IsSet(slot) {
		c.tags[slot] = tag
		c.viewIndexes[tag] = append(c.viewIndexes[tag], slot)
		c.setslots.Set(slot)
		c.setcnt++
	}
}

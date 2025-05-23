package seekindex

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Entry struct {
	Min    super.Value `super:"min"`
	Max    super.Value `super:"max"`
	ValOff uint64      `super:"val_off"`
	ValCnt uint64      `super:"val_cnt"`
	Offset uint64      `super:"offset"`
	Length uint64      `super:"length"`
}

func (e Entry) Range() Range {
	return Range{
		Offset: int64(e.Offset),
		Length: int64(e.Length),
	}
}

type Index []Entry

func (i Index) Filter(b *vector.Bool) Ranges {
	var ranges Ranges
	for _, e := range i {
		for off := e.ValOff; off < uint64(b.Len()) && off < e.ValOff+e.ValCnt; off++ {
			if b.IsSet(uint32(off)) {
				ranges.Append(e)
				break
			}
		}
	}
	return ranges
}

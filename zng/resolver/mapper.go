package resolver

import (
	"github.com/mccanne/zq/zbuf"
	"github.com/mccanne/zq/zng"
)

type Mapper struct {
	Slice
	out *Table
}

func NewMapper(out *Table) *Mapper {
	return &Mapper{out: out}
}

// Map maps an input side descriptor ID to an output side descriptor.
// The outputs are stored in a Table, which will create a new decriptor if
// the type is unknown to it.  The output side is assumed to be shared
// while the input side owned by one thread of control.
func (m *Mapper) Map(td int) *zbuf.Descriptor {
	return m.lookup(td)
}

func (m *Mapper) Enter(td int, typ *zng.TypeRecord) *zbuf.Descriptor {
	if dout := m.out.GetByValue(typ); dout != nil {
		m.enter(td, dout)
		return dout
	}
	return nil
}

func (m *Mapper) EnterDescriptor(td int, d *zbuf.Descriptor) {
	m.enter(td, d)
}

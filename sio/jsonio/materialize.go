package jsonio

import "github.com/brimdata/super/vector"

func Materialize(b *Builder) vector.Any {
	materializeValue(b.root)
	return nil
}

func materializeValue(c *Value) vector.Any {
	if c.Object != nil {
		materializeRecord(c.Object)
	}
	return nil
}

func materializeRecord(o *Record) vector.Any {
	// The end of which we build this but with records:
	// type Dynamic struct {
	// 	Tags   []uint32
	// 	Values []Any
	// }
	var vecs []vector.Any
	for _, idx := range o.lut {
		col := o.fields[idx]
		vec := materializeValue(col)
		vecs = append(vecs, vec)
	}
	// XXX Builder and return dynamic here.
	return vecs[0]
}

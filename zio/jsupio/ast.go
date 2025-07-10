package jsupio

import (
	"github.com/brimdata/super/pkg/unpack"
)

var unpacker = make(unpack.Reflector)

func init() {
	unpacker.AddAs(zPrimitive{}, "primitive")
	unpacker.AddAs(zRecord{}, "record")
	unpacker.AddAs(zArray{}, "array")
	unpacker.AddAs(zSet{}, "set")
	unpacker.AddAs(zMap{}, "map")
	unpacker.AddAs(zUnion{}, "union")
	unpacker.AddAs(zEnum{}, "enum")
	unpacker.AddAs(zError{}, "error")
	unpacker.AddAs(zNamed{}, "named")
	unpacker.AddAs(zRef{}, "ref")
}

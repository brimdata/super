package function

import (
	"github.com/brimdata/super/vector"
)

type defuse struct{}

func (defuse) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }

func (defuse) Call(args ...vector.Any) vector.Any {
	return args[0]
}

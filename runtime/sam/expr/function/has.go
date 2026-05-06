package function

import (
	"github.com/brimdata/super"
)

type Has struct{}

func (h *Has) Call(args []super.Value) super.Value {
	args = underAll(args)
	for _, val := range args {
		if val.IsNull() {
			return super.Null
		}
	}
	for _, val := range args {
		if val.IsError() {
			if val.IsMissing() || val.IsQuiet() {
				return super.False
			}
			return val
		}
		if val.IsNone() {
			return super.False
		}
	}
	return super.True
}

type Missing struct {
	has Has
}

func (m *Missing) Call(args []super.Value) super.Value {
	val := m.has.Call(args)
	if val.Type() == super.TypeBool && !val.IsNull() {
		return super.NewBool(!val.Bool())
	}
	return val
}

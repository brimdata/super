package function

import "github.com/brimdata/super"

type Has struct{}

func (h *Has) Call(args []super.Value) super.Value {
	for _, val := range args {
		if val.IsNull() {
			return super.NullBool
		}
		if val.IsError() {
			if val.IsMissing() || val.IsQuiet() {
				return super.False
			}
			return val
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

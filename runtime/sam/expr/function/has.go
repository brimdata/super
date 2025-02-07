package function

import "github.com/brimdata/super"

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#has
type Has struct{}

func (h *Has) Call(_ super.Allocator, args []super.Value) super.Value {
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

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#missing
type Missing struct {
	has Has
}

func (m *Missing) Call(ectx super.Allocator, args []super.Value) super.Value {
	val := m.has.Call(ectx, args)
	if val.Type() == super.TypeBool && !val.IsNull() {
		return super.NewBool(!val.Bool())
	}
	return val
}

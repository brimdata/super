package function

import "github.com/brimdata/super"

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#coalesce
type Coalesce struct{}

func (c *Coalesce) Call(_ super.Allocator, args []super.Value) super.Value {
	for i := range args {
		val := args[i].Under()
		if !val.IsNull() && !val.IsMissing() && !val.IsQuiet() {
			return args[i]
		}
	}
	return super.Null
}

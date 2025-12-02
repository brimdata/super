package function

import "github.com/brimdata/super"

type Coalesce struct{}

func (c *Coalesce) Call(args []super.Value) super.Value {
	for i := range args {
		val := args[i].Under()
		if !val.IsNull() && !val.IsError() {
			return args[i]
		}
	}
	return super.Null
}

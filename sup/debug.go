package sup

import (
	"fmt"

	"github.com/superdb/super"
)

func init() {
	super.Debug = func(args ...any) {
		var out []any
		for _, arg := range args {
			switch arg.(type) {
			case super.Type, *super.Value, super.Value:
				out = append(out, String(arg))
			default:
				out = append(out, arg)
			}
		}
		fmt.Println(out...)
	}
}

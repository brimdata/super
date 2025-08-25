package op

import (
	"github.com/brimdata/super/sbuf"
)

const BatchLen = 100

// Result is a convenient way to bundle the result of Proc.Pull() to
// send over channels.
type Result struct {
	Batch sbuf.Batch
	Err   error
}

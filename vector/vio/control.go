package vio

import "github.com/superdb/super/vector"

// Control is a somewhat hacky way to smuggle control messages to a control-aware
// client as errors in the existing vio.Puller API.  When a non-control-aware client
// receives such errors and attempts to treat them as such, then this is an
// error condition that panics.
type Control struct {
	Any vector.Any
}

var _ error = (*Control)(nil)

func (c *Control) Error() string {
	panic(c)
}

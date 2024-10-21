package expr

import (
	"github.com/brimdata/super"
)

// Context is an interface to a scope and value allocator for expressions.
// This allows external packages to implement efficient temporary buffering
// of Zed values both for let-style temporary variables accessible via
// the scope and for allocating results.
type Context interface {
	// Vars() accesses the variables reachable in the current scope.
	Vars() []super.Value
	super.Allocator
}

type allocator struct{}

var _ Context = (*allocator)(nil)

func NewContext() *allocator {
	return &allocator{}
}

func (*allocator) Vars() []super.Value {
	return nil
}

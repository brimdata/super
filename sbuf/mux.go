package sbuf

import (
	"github.com/brimdata/super"
)

func Label(label string, batch Batch) Batch {
	return &labeled{batch, label}
}

func Unlabel(batch Batch) (Batch, string) {
	var label string
	if inner, ok := batch.(*labeled); ok {
		batch = inner
		label = inner.label
	}
	return batch, label
}

type labeled struct {
	Batch
	label string
}

// EndOfChannel is an empty batch that represents the termination of one
// of the output paths of a muxed flowgraph and thus will be ignored downstream
// unless explicitly detected.
type EndOfChannel string

var _ Batch = (*EndOfChannel)(nil)

func (*EndOfChannel) Ref()                  {}
func (*EndOfChannel) Unref()                {}
func (*EndOfChannel) Values() []super.Value { return nil }

package zbuf

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
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

func CopyMux(outputs map[string]zio.WriteCloser, parent Puller) error {
	for {
		batch, err := parent.Pull(false)
		if batch == nil || err != nil {
			return err
		}
		if _, ok := batch.(*EndOfChannel); ok {
			continue
		}
		var label string
		batch, label = Unlabel(batch)
		if w, ok := outputs[label]; ok {
			if err := WriteBatch(w, batch); err != nil {
				return err
			}
		}
		// XXX Should we send some kind of warning if a batch is ignored
		// because it doesn't have an assigned output channel?
		batch.Unref()
	}
}

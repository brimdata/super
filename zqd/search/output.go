package search

import (
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zqd/api"
)

type Output interface {
	SendBatch(int, zbuf.Batch) error
	SendControl(interface{}) error
	End(interface{}) error
	ContentType() string
}

func SendFromReader(out Output, r zbuf.Reader) (err error) {
	if err = out.SendControl(&api.TaskStart{"TaskStart", 0}); err != nil {
		return
	}
	defer func() {
		if err != nil {
			verr := api.Error{Type: "INTERNAL", Message: err.Error()}
			out.End(&api.TaskEnd{"TaskEnd", 0, &verr})
			return
		}
		err = out.End(&api.TaskEnd{"TaskEnd", 0, nil})
	}()

	for {
		var b zbuf.Batch
		if b, err = zbuf.ReadBatch(r, DefaultMTU); err != nil {
			return
		}
		if b == nil {
			return
		}
		if err = out.SendBatch(0, b); err != nil {
			return
		}
	}
}

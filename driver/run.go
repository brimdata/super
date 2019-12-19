package driver

import (
	"fmt"
	"io"
	"time"

	"github.com/mccanne/zq/pkg/zng"
	"github.com/mccanne/zq/proc"
)

type Driver struct {
	writer   zng.Writer
	warnings io.Writer
}

func New(w zng.Writer) *Driver {
	return &Driver{
		writer: w,
	}
}

func (d *Driver) SetWarningsWriter(w io.Writer) {
	d.warnings = w
}

func (d *Driver) Write(cid int, arr zng.Batch) error {
	for _, r := range arr.Records() {
		if err := d.writer.Write(r); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) WriteWarning(msg string) error {
	if d.warnings != nil {
		_, err := fmt.Fprintln(d.warnings, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) Run(out *proc.MuxOutput) error {
	for !out.Complete() {
		res := out.Pull(time.After(time.Second * 10))
		if res.Err == proc.ErrTimeout {
			continue
		}
		if res.Err != nil {
			return res.Err
		}
		if res.Warning != "" {
			d.WriteWarning(res.Warning)
		}
		if res.Batch != nil {
			if err := d.Write(res.ID, res.Batch); err != nil {
				return err
			}
		}
	}
	return nil
}

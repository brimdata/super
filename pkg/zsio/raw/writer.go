package raw

import (
	"encoding/json"
	"io"

	"github.com/mccanne/zq/pkg/zson"
)

type Raw struct {
	io.Writer
}

func NewWriter(w io.Writer) *Raw {
	return &Raw{Writer: w}
}

func (r *Raw) WriteRaw(msg interface{}) error {
	out, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	out = append(out, byte('\n'))
	_, err = r.Writer.Write(out)
	return err
}

//XXX ?
func (r *Raw) Write(rec *zson.Record) error {
	return nil
}

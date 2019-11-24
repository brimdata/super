package json

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/mccanne/zq/pkg/zson"
)

// JSON implements a Formatter for json output
type JSON struct {
	io.Writer
	limit int
	array []map[string]interface{}
}

func NewWriter(w io.Writer) *JSON {
	return &JSON{Writer: w, limit: 10000}
}

func (p *JSON) Write(rec *zson.Record) error {
	return errors.New("not yet implemented")
	// XXX not yet...
	// td from column 0 has been stripped out
	// object := makeJSON(d, t)
	// if len(p.array) >= p.limit {
	// return ErrTooManyLines
	// }
	// p.array = append(p.array, object)
	return nil
}

func (p *JSON) Flush() error {
	out, err := json.Marshal(p.array)
	if err != nil {
		return err
	}
	_, err = p.Writer.Write(out)
	return err
}

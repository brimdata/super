package search

import (
	"net/http"

	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/csvio"
	"github.com/brimdata/zed/zson"
)

// CSVOutput implements the Output inteface and writes csv encoded-output
// directly to the client as text/csv.
type CSVOutput struct {
	response http.ResponseWriter
	wc       zbuf.WriteCloser
}

func NewCSVOutput(response http.ResponseWriter, ctrl bool) *CSVOutput {
	return &CSVOutput{
		response: response,
		wc: csvio.NewWriter(zio.NopCloser(response), zson.NewContext(), csvio.WriterOpts{
			Fuse: true,
			UTF8: true,
		}),
	}
}

func (r *CSVOutput) Collect() interface{} {
	return "TBD" //XXX
}

func (r *CSVOutput) SendBatch(cid int, batch zbuf.Batch) error {
	for _, rec := range batch.Records() {
		if err := r.wc.Write(rec); err != nil {
			return err
		}
	}
	batch.Unref()
	return nil
}

func (r *CSVOutput) End(ctrl interface{}) error {
	return r.wc.Close()
}

func (r *CSVOutput) SendControl(ctrl interface{}) error {
	return nil
}

func (r *CSVOutput) ContentType() string {
	return MimeTypeCSV
}

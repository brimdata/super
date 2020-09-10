package search

import (
	"encoding/json"
	"net/http"

	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/zngio"
)

// ZngOutput writes zng encodings directly to the client via
// binary data sent over http chunked encoding interleaved with json
// protocol messages sent as zng comment payloads.  The simplicity of
// this is a thing of beauty.
// Also, it implements the Output interface.
type ZngOutput struct {
	response http.ResponseWriter
	writer   *zngio.Writer
	ctrl     bool
}

func NewZngOutput(response http.ResponseWriter, ctrl bool) *ZngOutput {
	o := &ZngOutput{
		response: response,
		writer:   zngio.NewWriter(zio.NopCloser(response), zio.WriterFlags{}),
		ctrl:     ctrl,
	}
	return o
}

func (r *ZngOutput) flush() {
	r.response.(http.Flusher).Flush()
}

func (r *ZngOutput) Collect() interface{} {
	return "TBD" //XXX
}

func (r *ZngOutput) SendBatch(cid int, batch zbuf.Batch) error {
	for _, rec := range batch.Records() {
		// XXX need to send channel id as control payload
		if err := r.writer.Write(rec); err != nil {
			return err
		}
	}
	batch.Unref()
	r.flush()
	return nil
}

func (r *ZngOutput) End(ctrl interface{}) error {
	return r.SendControl(ctrl)
}

func (r *ZngOutput) SendControl(ctrl interface{}) error {
	if !r.ctrl {
		return nil
	}
	msg, err := json.Marshal(ctrl)
	if err != nil {
		//XXX need a better json error message
		return err
	}
	b := []byte("json:")
	if err := r.writer.WriteControl(append(b, msg...)); err != nil {
		return err
	}
	r.flush()
	return nil
}

func (r *ZngOutput) ContentType() string {
	return MimeTypeZNG
}

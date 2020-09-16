package search

import (
	"net/http"

	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio/zjsonio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zqd/api"
)

// ZJSON implements the Output interface.
type ZJSON struct {
	pipe   *api.JSONPipe
	stream *zjsonio.Stream
	mtu    int
	ctrl   bool
}

func NewZJSONOutput(resp http.ResponseWriter, mtu int, ctrl bool) *ZJSON {
	return &ZJSON{
		pipe:   api.NewJSONPipe(resp),
		stream: zjsonio.NewStream(),
		mtu:    mtu,
		ctrl:   ctrl,
	}
}

func formatRecords(stream *zjsonio.Stream, records []*zng.Record) ([]zjsonio.Record, error) {
	var res = make([]zjsonio.Record, len(records))
	for i, in := range records {
		out, err := stream.Transform(in)
		if err != nil {
			return nil, err
		}
		res[i] = *out
	}
	return res, nil
}

func (s *ZJSON) SendBatch(cid int, set zbuf.Batch) error {
	records := set.Records()
	n := len(records)
	for n > 0 {
		frag := n
		if frag > s.mtu {
			frag = s.mtu
		}
		formatted, err := formatRecords(s.stream, records[0:frag])
		if err != nil {
			return err
		}
		v := &api.SearchRecords{
			Type:      "SearchRecords",
			ChannelID: cid,
			Records:   formatted,
		}
		if err := s.pipe.Send(v); err != nil {
			return err
		}
		records = records[frag:]
		n -= frag
	}
	set.Unref()
	return nil
}

func (s *ZJSON) SendControl(msg interface{}) error {
	if !s.ctrl {
		return nil
	}
	return s.pipe.Send(msg)
}

func (s *ZJSON) End(msg interface{}) error {
	if !s.ctrl {
		msg = nil
	}
	return s.pipe.SendFinal(msg)
}

func (s *ZJSON) ContentType() string {
	return MimeTypeZJSON
}

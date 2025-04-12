package client

import (
	"fmt"
	"io"

	"github.com/brimdata/super/api"
	"github.com/brimdata/super/sup"
)

type EventsClient struct {
	rc          io.ReadCloser
	unmarshaler *sup.UnmarshalContext
}

func newEventsClient(resp *Response) *EventsClient {
	unmarshaler := sup.NewUnmarshaler()
	unmarshaler.Bind(
		api.EventPool{},
		api.EventBranch{},
		api.EventBranchCommit{},
	)
	return &EventsClient{
		rc:          resp.Body,
		unmarshaler: unmarshaler,
	}
}

func (l *EventsClient) Recv() (string, any, error) {
	var kind, data string
	_, err := fmt.Fscanf(l.rc, "event: %s\ndata: %s\n\n\n", &kind, &data)
	if err != nil {
		return "", nil, err
	}
	var v any
	if err := l.unmarshaler.Unmarshal(data, &v); err != nil {
		return "", nil, err
	}
	return kind, v, err
}

func (l *EventsClient) Close() error {
	return l.rc.Close()
}

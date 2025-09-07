package queryio

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
)

type scanner struct {
	channel  string
	scanner  sbuf.Scanner
	closer   io.Closer
	progress sbuf.Progress
}

func NewScanner(ctx context.Context, rc io.ReadCloser) (sbuf.Scanner, error) {
	s, err := bsupio.NewReader(super.NewContext(), rc).NewScanner(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &scanner{
		scanner: s,
		closer:  rc,
	}, nil
}

func (s *scanner) Progress() sbuf.Progress {
	return s.progress
}

func (s *scanner) Pull(done bool) (sbuf.Batch, error) {
again:
	batch, err := s.scanner.Pull(done)
	if err == nil {
		if batch != nil {
			return sbuf.Label(s.channel, batch), nil
		}
		return nil, s.closer.Close()
	}
	sctrl, ok := err.(*sbuf.Control)
	if !ok {
		return nil, err
	}
	ctrl, err := marshalControl(sctrl)
	if err != nil {
		return nil, err
	}
	switch ctrl := ctrl.(type) {
	case *api.QueryChannelSet:
		s.channel = ctrl.Channel
		goto again
	case *api.QueryChannelEnd:
		eoc := sbuf.EndOfChannel(ctrl.Channel)
		return &eoc, nil
	case *api.QueryStats:
		s.progress.Add(ctrl.Progress)
		goto again
	case *api.QueryError:
		return nil, errors.New(ctrl.Error)
	default:
		return nil, fmt.Errorf("unsupported control message: %T", ctrl)
	}
}

func marshalControl(zctrl *sbuf.Control) (any, error) {
	ctrl, ok := zctrl.Message.(*bsupio.Control)
	if !ok {
		return nil, fmt.Errorf("unknown control type: %T", zctrl.Message)
	}
	if ctrl.Format != bsupio.ControlFormatSUP {
		return nil, fmt.Errorf("unsupported app encoding: %v", ctrl.Format)
	}
	value, err := sup.ParseValue(super.NewContext(), string(ctrl.Bytes))
	if err != nil {
		return nil, fmt.Errorf("unable to parse control message: %w (%s)", err, ctrl.Bytes)
	}
	var v any
	if err := unmarshaler.Unmarshal(value, &v); err != nil {
		return nil, fmt.Errorf("unable to unmarshal control message: %w (%s)", err, ctrl.Bytes)
	}
	return v, nil
}

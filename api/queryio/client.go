package queryio

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zbuf"
)

type scanner struct {
	channel  string
	scanner  zbuf.Scanner
	closer   io.Closer
	progress zbuf.Progress
}

func NewScanner(ctx context.Context, rc io.ReadCloser) (zbuf.Scanner, error) {
	s, err := bsupio.NewReader(super.NewContext(), rc).NewScanner(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &scanner{
		scanner: s,
		closer:  rc,
	}, nil
}

func (s *scanner) Progress() zbuf.Progress {
	return s.progress
}

func (s *scanner) Pull(done bool) (zbuf.Batch, error) {
again:
	batch, err := s.scanner.Pull(done)
	if err == nil {
		if batch != nil {
			return zbuf.Label(s.channel, batch), nil
		}
		return nil, s.closer.Close()
	}
	zctrl, ok := err.(*zbuf.Control)
	if !ok {
		return nil, err
	}
	v, err := marshalControl(zctrl)
	if err != nil {
		return nil, err
	}
	switch ctrl := v.(type) {
	case *api.QueryChannelSet:
		s.channel = ctrl.Channel
		goto again
	case *api.QueryChannelEnd:
		eoc := zbuf.EndOfChannel(ctrl.Channel)
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

func marshalControl(zctrl *zbuf.Control) (any, error) {
	ctrl, ok := zctrl.Message.(*bsupio.Control)
	if !ok {
		return nil, fmt.Errorf("unknown control type: %T", zctrl.Message)
	}
	if ctrl.Format != bsupio.ControlFormatSUP {
		return nil, fmt.Errorf("unsupported app encoding: %v", ctrl.Format)
	}
	value, err := sup.ParseValue(super.NewContext(), string(ctrl.Bytes))
	if err != nil {
		return nil, fmt.Errorf("unable to parse Zed control message: %w (%s)", err, ctrl.Bytes)
	}
	var v any
	if err := unmarshaler.Unmarshal(value, &v); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Zed control message: %w (%s)", err, ctrl.Bytes)
	}
	return v, nil
}

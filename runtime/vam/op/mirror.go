package op

import (
	"context"

	"github.com/brimdata/super/vector"
)

type Mirror struct {
	ctx    context.Context
	parent vector.Puller

	blocked  bool
	mirrored *mirrored
}

func NewMirror(ctx context.Context, parent vector.Puller) *Mirror {
	return &Mirror{
		ctx:    ctx,
		parent: parent,
		mirrored: &mirrored{
			ctx:      ctx,
			doneCh:   make(chan struct{}),
			resultCh: make(chan result),
		},
	}
}

func (m *Mirror) Pull(done bool) (vector.Any, error) {
	vec, err := m.parent.Pull(done)
	if vec == nil || err != nil {
		m.sendEOS(err)
		return vec, err
	}
	if !m.blocked {
		select {
		case m.mirrored.resultCh <- result{vec, nil}:
		case <-m.mirrored.doneCh:
			m.blocked = true
		case <-m.ctx.Done():
			return nil, m.ctx.Err()
		}
	}
	return vec, err
}

func (m *Mirror) sendEOS(err error) {
	if !m.blocked {
		select {
		case m.mirrored.resultCh <- result{nil, err}:
			m.blocked = true
		case <-m.mirrored.doneCh:
			m.blocked = true
		case <-m.ctx.Done():
		}
	}
	m.blocked = false
}

func (m *Mirror) Mirrored() vector.Puller {
	return m.mirrored
}

type mirrored struct {
	ctx      context.Context
	doneCh   chan struct{}
	resultCh chan result
}

func (m *mirrored) Pull(done bool) (vector.Any, error) {
	if done {
		select {
		case m.doneCh <- struct{}{}:
			return nil, nil
		case <-m.ctx.Done():
			return nil, m.ctx.Err()
		}
	}
	select {
	case r := <-m.resultCh:
		return r.vector, r.err
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	}
}

package vector

import (
	"io"
	"sync/atomic"
)

type Puller interface {
	Pull(done bool) (Any, error)
}

// A Scanner is a Puller that also provides progress updates.
type Scanner interface {
	Meter
	Puller
}

type Writer interface {
	Write(Any) error
}

type WriteCloser interface {
	Writer
	io.Closer
}

func NewPuller(vecs ...Any) Puller {
	return &puller{vecs}
}

type puller struct {
	vecs []Any
}

func (p *puller) Pull(done bool) (Any, error) {
	if len(p.vecs) == 0 {
		return nil, nil
	}
	vec := p.vecs[0]
	p.vecs = p.vecs[1:]
	return vec, nil
}

type Labeled struct {
	Any
	Label string
}

func Unlabel(vec Any) (Any, string) {
	if vec, ok := vec.(*Labeled); ok {
		return vec.Any, vec.Label
	}
	return vec, ""
}

// A Meter provides Progress statistics.
type Meter interface {
	Progress() Progress
}

// Progress represents progress statistics from a Scanner.
type Progress struct {
	BytesRead      int64 `super:"bytes_read" json:"bytes_read"`
	BytesMatched   int64 `super:"bytes_matched" json:"bytes_matched"`
	RecordsRead    int64 `super:"records_read" json:"records_read"`
	RecordsMatched int64 `super:"records_matched" json:"records_matched"`
}

var _ Meter = (*Progress)(nil)

// Add updates its receiver by adding to it the values in ss.
func (p *Progress) Add(in Progress) {
	if p != nil {
		atomic.AddInt64(&p.BytesRead, in.BytesRead)
		atomic.AddInt64(&p.BytesMatched, in.BytesMatched)
		atomic.AddInt64(&p.RecordsRead, in.RecordsRead)
		atomic.AddInt64(&p.RecordsMatched, in.RecordsMatched)
	}
}

func (p *Progress) Copy() Progress {
	if p == nil {
		return Progress{}
	}
	return Progress{
		BytesRead:      atomic.LoadInt64(&p.BytesRead),
		BytesMatched:   atomic.LoadInt64(&p.BytesMatched),
		RecordsRead:    atomic.LoadInt64(&p.RecordsRead),
		RecordsMatched: atomic.LoadInt64(&p.RecordsMatched),
	}
}

func (p *Progress) Progress() Progress {
	return p.Copy()
}

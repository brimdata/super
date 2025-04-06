package zbuf

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zio"
)

type Pushdown interface {
	Projection() field.Projection
	DataFilter() (expr.Evaluator, error)
	BSUPFilter() (*expr.BufferFilter, error)
	MetaFilter() (expr.Evaluator, field.Projection, error)
}

// ScannerAble is implemented by Readers that provide an optimized
// implementation of the Scanner interface.
type ScannerAble interface {
	NewScanner(context.Context, Pushdown) (Scanner, error)
}

// A Meter provides Progress statistics.
type Meter interface {
	Progress() Progress
}

// A Scanner is a Batch source that also provides progress updates.
type Scanner interface {
	Meter
	Puller
}

// Progress represents progress statistics from a Scanner.
type Progress struct {
	BytesRead      int64 `zed:"bytes_read" json:"bytes_read"`
	BytesMatched   int64 `zed:"bytes_matched" json:"bytes_matched"`
	RecordsRead    int64 `zed:"records_read" json:"records_read"`
	RecordsMatched int64 `zed:"records_matched" json:"records_matched"`
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

// NewScanner returns a Scanner for r that filters records by filterExpr and s.
// If r implements fmt.Stringer, the scanner reports errors using a prefix of the
// string returned by its String method.
func NewScanner(ctx context.Context, r zio.Reader, filterExpr Pushdown) (Scanner, error) {
	s, err := newScanner(ctx, r, filterExpr)
	if err != nil {
		return nil, err
	}
	if stringer, ok := r.(fmt.Stringer); ok {
		s = NamedScanner(s, stringer.String())
	}
	return s, nil
}

func newScanner(ctx context.Context, r zio.Reader, filterExpr Pushdown) (Scanner, error) {
	var sa ScannerAble
	if zf, ok := r.(*File); ok {
		sa, _ = zf.Reader.(ScannerAble)
	} else {
		sa, _ = r.(ScannerAble)
	}
	if sa != nil {
		return sa.NewScanner(ctx, filterExpr)
	}
	var f expr.Evaluator
	if filterExpr != nil {
		var err error
		if f, err = filterExpr.DataFilter(); err != nil {
			return nil, err
		}
	}
	sc := &scanner{reader: r, filter: f, ctx: ctx, ectx: expr.NewContext()}
	sc.Puller = NewPuller(sc)
	return sc, nil
}

type scanner struct {
	Puller
	reader   zio.Reader
	filter   expr.Evaluator
	ctx      context.Context
	ectx     expr.Context
	progress Progress
}

func (s *scanner) Progress() Progress {
	return s.progress.Copy()
}

// Read implements Reader.Read.
func (s *scanner) Read() (*super.Value, error) {
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		this, err := s.reader.Read()
		if err != nil || this == nil {
			return nil, err
		}
		atomic.AddInt64(&s.progress.BytesRead, int64(len(this.Bytes())))
		atomic.AddInt64(&s.progress.RecordsRead, 1)
		if s.filter != nil {
			val := s.filter.Eval(s.ectx, *this)
			if !(val.Type() == super.TypeBool && val.Bool()) {
				continue
			}
		}
		atomic.AddInt64(&s.progress.BytesMatched, int64(len(this.Bytes())))
		atomic.AddInt64(&s.progress.RecordsMatched, 1)
		return this, nil
	}
}

type MultiStats []Scanner

func (m MultiStats) Progress() Progress {
	var ss Progress
	for _, s := range m {
		ss.Add(s.Progress())
	}
	return ss
}

func NamedScanner(s Scanner, name string) *namedScanner {
	return &namedScanner{
		Scanner: s,
		name:    name,
	}
}

type namedScanner struct {
	Scanner
	name string
}

func (n *namedScanner) Pull(done bool) (Batch, error) {
	b, err := n.Scanner.Pull(done)
	if err != nil {
		err = fmt.Errorf("%s: %w", n.name, err)
	}
	return b, err
}

func MultiScanner(scanners ...Scanner) Scanner {
	return &multiScanner{scanners: scanners}
}

type multiScanner struct {
	scanners []Scanner
	progress Progress
}

func (m *multiScanner) Pull(done bool) (Batch, error) {
	for len(m.scanners) > 0 {
		batch, err := m.scanners[0].Pull(done)
		if batch != nil || err != nil {
			return batch, err
		}
		m.progress.Add(m.scanners[0].Progress())
		m.scanners = m.scanners[1:]
	}
	return nil, nil
}

func (m *multiScanner) Progress() Progress {
	return m.progress.Copy()
}

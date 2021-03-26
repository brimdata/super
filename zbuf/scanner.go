package zbuf

import (
	"context"
	"sync/atomic"

	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zng"
)

type Filter interface {
	AsFilter() (expr.Filter, error)
	AsBufferFilter() (*expr.BufferFilter, error)
}

// ScannerAble is implemented by Readers that provide an optimized
// implementation of the Scanner interface.
type ScannerAble interface {
	NewScanner(ctx context.Context, filterExpr Filter, s nano.Span) (Scanner, error)
}

// A Statser produces scanner statistics.
type Statser interface {
	Stats() *ScannerStats
}

// A Scanner is a Batch source that also provides statistics.
type Scanner interface {
	Statser
	Puller
}

// ScannerStats holds Scanner statistics. It should be identical to
// api.ScannerStats.
type ScannerStats struct {
	BytesRead      int64
	BytesMatched   int64
	RecordsRead    int64
	RecordsMatched int64
}

// Accumulate updates its receiver by adding to it the values in ss.
func (s *ScannerStats) Accumulate(ss *ScannerStats) {
	s.BytesRead += ss.BytesRead
	s.BytesMatched += ss.BytesMatched
	s.RecordsRead += ss.RecordsRead
	s.RecordsMatched += ss.RecordsMatched
}

func ReadersToScanners(ctx context.Context, readers []Reader) ([]Scanner, error) {
	scanners := make([]Scanner, 0, len(readers))
	for _, reader := range readers {
		s, err := NewScanner(ctx, reader, nil, nano.MaxSpan)
		if err != nil {
			return nil, err
		}
		scanners = append(scanners, s)
	}
	return scanners, nil
}

// ReadersToPullers returns a slice of Pullers that pull from the given
// Readers.  If any or all of the readers implement Scannerable, then
// a scanner will be created from the underlying Scannerable so that the
// pulled Batches are more efficient, i.e., the zng scanner will arrange
// for each Batch to be returned to a pool instead of being GC'd.
func ReadersToPullers(ctx context.Context, readers []Reader) ([]Puller, error) {
	scanners, err := ReadersToScanners(ctx, readers)
	if err != nil {
		return nil, err
	}
	pullers := make([]Puller, 0, len(scanners))
	for _, s := range scanners {
		pullers = append(pullers, s)
	}
	return pullers, nil
}

var ScannerBatchSize = 100

// NewScanner returns a Scanner for r that filters records by filterExpr and s.
func NewScanner(ctx context.Context, r Reader, filterExpr Filter, s nano.Span) (Scanner, error) {
	var sa ScannerAble
	if zf, ok := r.(*File); ok {
		sa, _ = zf.Reader.(ScannerAble)
	} else {
		sa, _ = r.(ScannerAble)
	}
	if sa != nil {
		return sa.NewScanner(ctx, filterExpr, s)
	}
	var f expr.Filter
	if filterExpr != nil {
		var err error
		if f, err = filterExpr.AsFilter(); err != nil {
			return nil, err
		}
	}
	sc := &scanner{reader: r, filter: f, span: s, ctx: ctx}
	sc.Puller = NewPuller(sc, ScannerBatchSize)
	return sc, nil
}

type scanner struct {
	Puller
	reader Reader
	filter expr.Filter
	span   nano.Span
	ctx    context.Context

	stats ScannerStats
}

func (s *scanner) Stats() *ScannerStats {
	return &ScannerStats{
		BytesRead:      atomic.LoadInt64(&s.stats.BytesRead),
		BytesMatched:   atomic.LoadInt64(&s.stats.BytesMatched),
		RecordsRead:    atomic.LoadInt64(&s.stats.RecordsRead),
		RecordsMatched: atomic.LoadInt64(&s.stats.RecordsMatched),
	}
}

// Read implements Reader.Read.
func (s *scanner) Read() (*zng.Record, error) {
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		rec, err := s.reader.Read()
		if err != nil || rec == nil {
			return nil, err
		}
		atomic.AddInt64(&s.stats.BytesRead, int64(len(rec.Bytes)))
		atomic.AddInt64(&s.stats.RecordsRead, 1)
		if s.span != nano.MaxSpan && !s.span.Contains(rec.Ts()) ||
			s.filter != nil && !s.filter(rec) {
			continue
		}
		atomic.AddInt64(&s.stats.BytesMatched, int64(len(rec.Bytes)))
		atomic.AddInt64(&s.stats.RecordsMatched, 1)
		// Copy the underlying buffer (if volatile) because next call to
		// reader.Next() may overwrite said buffer.
		rec.CopyBytes()
		return rec, nil
	}
}

type MultiStats []Scanner

func (m MultiStats) Stats() *ScannerStats {
	var ss ScannerStats
	for _, s := range m {
		ss.Accumulate(s.Stats())
	}
	return &ss
}

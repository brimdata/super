// Package search provides an implementation for launching zq searches and performing
// analytics on zng files stored in the server's root directory.
package search

import (
	"context"
	"fmt"
	"time"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/ppl/zqd/storage"
	"github.com/brimdata/zed/ppl/zqd/storage/archivestore"
	"github.com/brimdata/zed/ppl/zqd/storage/filestore"
	"github.com/brimdata/zed/ppl/zqd/worker"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zng/resolver"
	"github.com/brimdata/zed/zqe"
	"go.uber.org/zap"
)

// This mtu is pretty small but it keeps the JSON object size below 64kb or so
// so the recevier can do reasonable, interactive streaming updates.
const DefaultMTU = 100

const StatsInterval = time.Millisecond * 500

const (
	MimeTypeCSV    = "text/csv"
	MimeTypeJSON   = "application/json"
	MimeTypeNDJSON = "application/x-ndjson"
	MimeTypeZJSON  = "application/x-zjson"
	MimeTypeZNG    = "application/x-zng"
)

type SearchOp struct {
	logger *zap.Logger
	query  *Query
}

func NewSearchOp(req api.SearchRequest, logger *zap.Logger) (*SearchOp, error) {
	if req.Span.Ts < 0 {
		return nil, zqe.ErrInvalid("time span must have non-negative timestamp")
	}
	if req.Span.Dur < 0 {
		return nil, zqe.ErrInvalid("time span must have non-negative duration")
	}
	// XXX zqd only supports backwards searches, remove once this has been
	// fixed.
	if req.Dir == 1 {
		return nil, zqe.ErrInvalid("forward searches not yet supported")
	}
	if req.Dir != -1 {
		return nil, zqe.ErrInvalid("time direction must be 1 or -1")
	}
	query, err := UnpackQuery(req)
	if err != nil {
		return nil, err
	}
	return &SearchOp{
		logger: logger,
		query:  query,
	}, nil
}

func (s *SearchOp) Run(ctx context.Context, store storage.Storage, output Output, parallelism int, wc worker.WorkerConfig) (err error) {
	d := &searchdriver{
		output:    output,
		startTime: nano.Now(),
	}
	d.start(0)
	defer func() {
		if err != nil {
			d.abort(0, err)
			return
		}
		d.end(0)
	}()

	statsTicker := time.NewTicker(StatsInterval)
	defer statsTicker.Stop()
	zctx := resolver.NewContext()

	switch st := store.(type) {
	case *archivestore.Storage:
		return driver.MultiRun(ctx, d, s.query.Proc, zctx, st.MultiSource(), driver.MultiConfig{
			Distributed: parallelism > 0,
			Logger:      s.logger,
			Order:       zbuf.OrderDesc,
			Parallelism: parallelism,
			Span:        s.query.Span,
			StatsTick:   statsTicker.C,
			Worker:      wc,
		})
	case *filestore.Storage:
		if parallelism > 1 {
			return fmt.Errorf("storage type %T does not support parallelism", st)
		}
		rc, err := st.Open(ctx, zctx, s.query.Span)
		if err != nil {
			return err
		}
		defer rc.Close()

		return driver.Run(ctx, d, s.query.Proc, zctx, rc, driver.Config{
			Logger:            s.logger,
			ReaderSortKey:     "ts",
			ReaderSortReverse: true,
			Span:              s.query.Span,
			StatsTick:         statsTicker.C,
		})
	default:
		return fmt.Errorf("unknown storage type %T", st)
	}
}

// A Query is the internal representation of search query describing a source
// of tuples, a "search" applied to the tuples producing a set of matched
// tuples, and a proc to the process the tuples
type Query struct {
	Space api.SpaceID
	Dir   int
	Span  nano.Span
	Proc  ast.Proc
}

// UnpackQuery transforms a api.SearchRequest into a Query.
func UnpackQuery(req api.SearchRequest) (*Query, error) {
	proc, err := ast.UnpackJSONAsProc(req.Proc)
	if err != nil {
		return nil, err
	}
	return &Query{
		Space: req.Space,
		Dir:   req.Dir,
		Span:  req.Span,
		Proc:  proc,
	}, nil
}

// searchdriver implements driver.Driver.
type searchdriver struct {
	output    Output
	startTime nano.Ts
}

func (d *searchdriver) start(id int64) error {
	return d.output.SendControl(&api.TaskStart{"TaskStart", id})
}

func (d *searchdriver) end(id int64) error {
	return d.output.End(&api.TaskEnd{"TaskEnd", id, nil})
}

func (d *searchdriver) abort(id int64, err error) error {
	verr := &api.Error{Type: "INTERNAL", Message: err.Error()}
	return d.output.SendControl(&api.TaskEnd{"TaskEnd", id, verr})
}

func (d *searchdriver) Warn(warning string) error {
	v := api.SearchWarning{
		Type:    "SearchWarning",
		Warning: warning,
	}
	return d.output.SendControl(v)
}

func (d *searchdriver) Write(cid int, batch zbuf.Batch) error {
	return d.output.SendBatch(cid, batch)
}

func (d *searchdriver) Stats(stats api.ScannerStats) error {
	v := api.SearchStats{
		Type:         "SearchStats",
		StartTime:    d.startTime,
		UpdateTime:   nano.Now(),
		ScannerStats: stats,
	}
	return d.output.SendControl(v)
}

func (d *searchdriver) ChannelEnd(cid int) error {
	v := &api.SearchEnd{
		Type:      "SearchEnd",
		ChannelID: cid,
		Reason:    "eof",
	}
	return d.output.SendControl(v)
}

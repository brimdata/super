package index

import (
	"context"
	"errors"
	"sync"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/index"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio"
)

func NewWriter(ctx context.Context, engine storage.Engine, path *storage.URI, object *Object) (*Writer, error) {
	rwCh := make(rwChan)
	indexer, err := newIndexer(ctx, engine, path, object, rwCh)
	if err != nil {
		return nil, err
	}
	w := &Writer{
		Object: object,
		URI:    object.Path(path),

		done:    make(chan struct{}),
		indexer: indexer,
		rwCh:    rwCh,
	}
	return w, nil
}

type Writer struct {
	Object *Object
	URI    *storage.URI

	done    chan struct{}
	indexer *indexer
	once    sync.Once
	rwCh    rwChan
}

type rwChan chan *zed.Record

func (c rwChan) Read() (*zed.Record, error) {
	return <-c, nil
}

func (w *Writer) Write(rec *zed.Record) error {
	select {
	case <-w.done:
		if err := w.indexer.err.Load(); err != nil {
			return err
		}
		return errors.New("index writer closed")
	default:
		w.once.Do(w.indexer.start)
		w.rwCh <- rec
		return nil
	}
}

func (w *Writer) Close() error {
	// If once has not be called, this means a write has never been called.
	// Abort index so no file is written.
	w.once.Do(func() {
		w.indexer.index.Abort()
	})
	close(w.done)
	close(w.rwCh)
	return w.indexer.Wait()
}

func (w *Writer) Abort() {
	w.Close()
	w.indexer.index.Abort()
}

// onceError is an object that will only store an error once.
type onceError struct {
	sync.Mutex // guards following
	err        error
}

func (a *onceError) Store(err error) {
	a.Lock()
	defer a.Unlock()
	if a.err != nil {
		return
	}
	a.err = err
}
func (a *onceError) Load() error {
	a.Lock()
	defer a.Unlock()
	return a.err
}

type indexer struct {
	err       onceError
	flowgraph zio.ReadCloser
	index     *index.Writer
	wg        sync.WaitGroup
}

func newIndexer(ctx context.Context, engine storage.Engine, path *storage.URI, object *Object, r zio.Reader) (*indexer, error) {
	rule := object.Rule
	zedQuery := rule.Zed()
	p, err := compiler.ParseProc(zedQuery)
	if err != nil {
		return nil, err
	}
	zctx := zed.NewContext()
	flowgraph, err := driver.NewReader(ctx, p, zctx, r)
	if err != nil {
		return nil, err
	}
	keys := rule.RuleKeys()
	writer, err := index.NewWriterWithContext(ctx, zctx, engine, object.Path(path).String(), keys)
	if err != nil {
		return nil, err
	}
	return &indexer{
		flowgraph: flowgraph,
		index:     writer,
	}, nil
}

func (d *indexer) start() {
	d.wg.Add(1)
	go func() {
		if err := zio.Copy(d, d.flowgraph); err != nil {
			d.index.Abort()
			d.err.Store(err)
		}
		d.err.Store(d.index.Close())
		d.wg.Done()
	}()
}

func (d *indexer) Wait() error {
	d.wg.Wait()
	return d.err.Load()
}

func (d *indexer) Write(rec *zed.Record) error {
	return d.index.Write(rec)
}

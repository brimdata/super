package zngio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"runtime"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
)

const (
	ReadSize  = 512 * 1024
	MaxSize   = 1024 * 1024 * 1024
	TypeLimit = 10000
)

type Reader struct {
	zctx    *super.Context
	reader  io.Reader
	opts    ReaderOpts
	scanner zbuf.Scanner
	wrap    zio.Reader
}

var _ zbuf.ScannerAble = (*Reader)(nil)

type ReaderOpts struct {
	Validate bool
	Size     int
	Max      int
	Threads  int
}

type Control struct {
	Format int
	Bytes  []byte
}

func NewReader(sctx *super.Context, reader io.Reader) *Reader {
	return NewReaderWithOpts(sctx, reader, ReaderOpts{})
}

func NewReaderWithOpts(zctx *super.Context, reader io.Reader, opts ReaderOpts) *Reader {
	if opts.Size == 0 {
		opts.Size = ReadSize
	}
	if opts.Max == 0 {
		opts.Max = MaxSize
	}
	if opts.Size > opts.Max {
		opts.Size = opts.Max
	}
	if opts.Threads == 0 {
		opts.Threads = runtime.GOMAXPROCS(0)
	}
	return &Reader{
		zctx:   zctx,
		reader: reader,
		opts:   opts,
	}
}

func (r *Reader) NewScanner(ctx context.Context, filter zbuf.Filter) (zbuf.Scanner, error) {
	if r.opts.Threads == 1 {
		return newScannerSync(ctx, r.zctx, r.reader, filter, r.opts)
	}
	return newScanner(ctx, r.zctx, r.reader, filter, r.opts)
}

// Close guarantees that the underlying io.Reader is not read after it returns.
func (r *Reader) Close() error {
	if r.scanner != nil {
		r.scanner.Pull(true)
	}
	return nil
}

func (r *Reader) init() error {
	if r.wrap != nil {
		return nil
	}
	//XXX ctx... seems like all NewReaders should take ctx so they
	// can have cancellable goroutines?
	scanner, err := r.NewScanner(context.TODO(), nil)
	if err != nil {
		return err
	}
	r.scanner = scanner
	r.wrap = zbuf.PullerReader(scanner)
	return nil
}

func (r *Reader) Read() (*super.Value, error) {
	// If Read is called, then this Reader is being used as a zio.Reader and
	// not as a zbuf.Puller.  We just wrap the scanner in a puller to
	// implement the Reader interface.  If it's used a zbuf.Scanner, then
	// the NewScanner method will be called and Read will never happen.
	if err := r.init(); err != nil {
		return nil, err
	}
	for {
		val, err := r.wrap.Read()
		if err != nil {
			if _, ok := err.(*zbuf.Control); ok {
				continue
			}
			return nil, err
		}
		return val, err
	}
}

func (r *Reader) ReadPayload() (*super.Value, *Control, error) {
	if err := r.init(); err != nil {
		return nil, nil, err
	}
	val, err := r.wrap.Read()
	if err != nil {
		if zctrl, ok := err.(*zbuf.Control); ok {
			ctrl, ok := zctrl.Message.(*Control)
			if !ok {
				return nil, nil, fmt.Errorf("zngio internal error: unknown control type: %T", zctrl.Message)
			}
			return nil, ctrl, nil
		}
	}
	return val, nil, err
}

func readUvarintAsInt(r io.ByteReader) (int, error) {
	u64, err := binary.ReadUvarint(r)
	return int(u64), err
}
